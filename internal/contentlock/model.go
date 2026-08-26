// Package contentlock owns user-facing content protection. It is intentionally
// separate from datalock, which only prevents concurrent writers.
package contentlock

import (
	"errors"
	"fmt"
	"time"
)

const (
	TargetSpace    = "space"
	TargetNotebook = "notebook"
	TargetNote     = "note"
	SpaceTargetID  = "__active_space__"

	ErrorCodeUnavailable         = "CONTENT_LOCK_UNAVAILABLE"
	ErrorCodeValidation          = "CONTENT_LOCK_VALIDATION"
	ErrorCodeAlreadyEnabled      = "CONTENT_LOCK_ALREADY_ENABLED"
	ErrorCodeNotFound            = "CONTENT_LOCK_NOT_FOUND"
	ErrorCodeLocked              = "CONTENT_LOCKED"
	ErrorCodePassphraseInvalid   = "CONTENT_LOCK_PASSPHRASE_INVALID"
	ErrorCodePassphraseRequired  = "CONTENT_LOCK_PASSPHRASE_REQUIRED"
	ErrorCodeAIConfirmation      = "CONTENT_LOCK_AI_RECORDS_CONFIRMATION_REQUIRED"
	ErrorCodeSyncDestination     = "CONTENT_LOCK_SYNC_DESTINATION_CHANGE_REQUIRED"
	ErrorCodeOperationInProgress = "CONTENT_LOCK_OPERATION_IN_PROGRESS"
	ErrorCodeIntegrity           = "CONTENT_LOCK_INTEGRITY"
)

var (
	ErrValidation               = errors.New("content lock validation failed")
	ErrAlreadyEnabled           = errors.New("content lock is already enabled")
	ErrNotFound                 = errors.New("content lock was not found")
	ErrLocked                   = errors.New("content is locked")
	ErrPassphraseInvalid        = errors.New("passphrase is invalid")
	ErrPassphraseRequired       = errors.New("passphrase is required")
	ErrAIRecordsRequireDeletion = errors.New("AI records must be deleted before locking content")
	ErrSyncDestinationChange    = errors.New("a new sync destination is required before locking content")
	ErrOperationInProgress      = errors.New("a content lock operation is in progress")
	ErrIntegrity                = errors.New("protected content integrity check failed")
)

type Target struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type Lock struct {
	ID         string    `json:"id"`
	TargetType string    `json:"targetType"`
	TargetID   string    `json:"targetId"`
	TargetName string    `json:"targetName"`
	Unlocked   bool      `json:"unlocked"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TargetStatus is intentionally metadata-only. It permits the UI to show a
// lock marker without exposing a passphrase, a key, or protected body text.
type TargetStatus struct {
	Protected    bool   `json:"protected"`
	Locked       bool   `json:"locked"`
	ExplicitLock bool   `json:"explicitLock"`
	Source       string `json:"source,omitempty"`
}

type ListResult struct {
	Locks []Lock    `json:"locks"`
	Error *APIError `json:"error,omitempty"`
}

type MutationResult struct {
	Lock            *Lock     `json:"lock,omitempty"`
	Removed         bool      `json:"removed"`
	Unlocked        bool      `json:"unlocked"`
	AIRecordCount   int       `json:"aiRecordCount,omitempty"`
	RestartRequired bool      `json:"restartRequired"`
	Error           *APIError `json:"error,omitempty"`
}

type EnableInput struct {
	TargetType      string `json:"targetType"`
	TargetID        string `json:"targetId"`
	Passphrase      string `json:"passphrase"`
	DeleteAIRecords bool   `json:"deleteAIRecords"`
}

type UnlockInput struct {
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Passphrase string `json:"passphrase"`
}

type ChangePassphraseInput struct {
	TargetType        string `json:"targetType"`
	TargetID          string `json:"targetId"`
	CurrentPassphrase string `json:"currentPassphrase"`
	NewPassphrase     string `json:"newPassphrase"`
}

type DisableInput struct {
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Passphrase string `json:"passphrase"`
}

type APIError struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	AIRecordCount int    `json:"aiRecordCount,omitempty"`
}

type aiRecordsRequiredError struct {
	count int
}

func (e *aiRecordsRequiredError) Error() string {
	return fmt.Sprintf("%s: %d", ErrAIRecordsRequireDeletion, e.count)
}

func (e *aiRecordsRequiredError) Unwrap() error { return ErrAIRecordsRequireDeletion }

func newAIRecordsRequiredError(count int) error {
	return &aiRecordsRequiredError{count: count}
}

func APIErrorFrom(err error) *APIError {
	if err == nil {
		return nil
	}
	var aiRecordsError *aiRecordsRequiredError
	if errors.As(err, &aiRecordsError) {
		return &APIError{
			Code:          ErrorCodeAIConfirmation,
			Message:       "関連するAI履歴・成果物を削除してからロックを設定してください。",
			AIRecordCount: aiRecordsError.count,
		}
	}
	switch {
	case errors.Is(err, ErrValidation):
		return &APIError{Code: ErrorCodeValidation, Message: "ロック対象または入力内容が正しくありません。"}
	case errors.Is(err, ErrAlreadyEnabled):
		return &APIError{Code: ErrorCodeAlreadyEnabled, Message: "この対象にはすでにロックが設定されています。"}
	case errors.Is(err, ErrNotFound):
		return &APIError{Code: ErrorCodeNotFound, Message: "ロック対象が見つかりません。"}
	case errors.Is(err, ErrLocked):
		return &APIError{Code: ErrorCodeLocked, Message: "この操作には対象ロックの解除が必要です。"}
	case errors.Is(err, ErrPassphraseRequired):
		return &APIError{Code: ErrorCodePassphraseRequired, Message: "パスフレーズを入力してください。"}
	case errors.Is(err, ErrPassphraseInvalid):
		return &APIError{Code: ErrorCodePassphraseInvalid, Message: "パスフレーズが正しくありません。"}
	case errors.Is(err, ErrSyncDestinationChange):
		return &APIError{Code: ErrorCodeSyncDestination, Message: "ロックを設定する前に既存の同期先を切断してください。保護済み本文の同期は、暗号化同期形式の導入まで利用できません。"}
	case errors.Is(err, ErrOperationInProgress):
		return &APIError{Code: ErrorCodeOperationInProgress, Message: "別のロック処理を完了しています。しばらくしてから再試行してください。"}
	case errors.Is(err, ErrIntegrity):
		return &APIError{Code: ErrorCodeIntegrity, Message: "暗号化された本文を検証できませんでした。内容は変更していません。"}
	default:
		return &APIError{Code: ErrorCodeUnavailable, Message: "ロックを利用できませんでした。データは変更していません。"}
	}
}
