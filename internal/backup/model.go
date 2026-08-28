package backup

import (
	"errors"
	"time"
)

const (
	KindAutomatic           = "automatic"
	KindRestoreSafety       = "restore-safety"
	manifestVersion         = 1
	maximumAutomaticBackups = 10
	maximumSafetyBackups    = 3
)

const backupInterval = 24 * time.Hour

var (
	ErrUnavailable          = errors.New("backup service is unavailable")
	ErrValidation           = errors.New("backup input is invalid")
	ErrNotFound             = errors.New("backup was not found")
	ErrTampered             = errors.New("backup verification failed")
	ErrRestoreAuthorization = errors.New("backup restore confirmation is invalid or expired")
	ErrRestorePending       = errors.New("a backup restore is already waiting for restart")
	ErrRecoveryConflict     = errors.New("another recovery operation is already waiting for restart")
	ErrRestoreApply         = errors.New("backup restore could not be applied")
	ErrAutomaticDegraded    = errors.New("automatic backup is unavailable while recovery is degraded")
	ErrAutomaticNotDue      = errors.New("automatic backup is not due")
)

type Paths struct {
	ManagementRoot string
	SpaceID        string
	DataDir        string
	DatabasePath   string
	NotesDir       string
}

type FileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Version       int         `json:"version"`
	ID            string      `json:"id"`
	Kind          string      `json:"kind"`
	SpaceID       string      `json:"spaceId"`
	CreatedAt     string      `json:"createdAt"`
	SchemaVersion int         `json:"schemaVersion"`
	TotalBytes    int64       `json:"totalBytes"`
	FileCount     int         `json:"fileCount"`
	Files         []FileEntry `json:"files"`
}

type BackupSummary struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	CreatedAt    string `json:"createdAt"`
	SizeBytes    int64  `json:"sizeBytes"`
	FileCount    int    `json:"fileCount"`
	Restorable   bool   `json:"restorable"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type ListResult struct {
	Backups []BackupSummary `json:"backups"`
	Error   *APIError       `json:"error,omitempty"`
}

type StatusResult struct {
	AutomaticEnabled bool      `json:"automaticEnabled"`
	AutomaticDue     bool      `json:"automaticDue"`
	LastAutomaticAt  string    `json:"lastAutomaticAt,omitempty"`
	BackupCount      int       `json:"backupCount"`
	PendingRestore   bool      `json:"pendingRestore"`
	Error            *APIError `json:"error,omitempty"`
}

type AutomaticResult struct {
	Created bool           `json:"created"`
	Skipped bool           `json:"skipped"`
	Backup  *BackupSummary `json:"backup,omitempty"`
	Error   *APIError      `json:"error,omitempty"`
}

type RestorePreview struct {
	Token     string `json:"token"`
	BackupID  string `json:"backupId"`
	CreatedAt string `json:"createdAt"`
	SizeBytes int64  `json:"sizeBytes"`
	FileCount int    `json:"fileCount"`
	Message   string `json:"message"`
}

type RestorePreviewResult struct {
	Preview *RestorePreview `json:"preview,omitempty"`
	Error   *APIError       `json:"error,omitempty"`
}

type RestoreExecutionInput struct {
	Token string `json:"token"`
}

type RestoreResult struct {
	BackupID              string    `json:"backupId,omitempty"`
	RestartRequired       bool      `json:"restartRequired"`
	RestoreSafetyBackupID string    `json:"restoreSafetyBackupId,omitempty"`
	Canceled              bool      `json:"canceled"`
	Message               string    `json:"message,omitempty"`
	Error                 *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	ErrorCodeUnavailable       = "BACKUP_UNAVAILABLE"
	ErrorCodeValidation        = "BACKUP_VALIDATION_FAILED"
	ErrorCodeNotFound          = "BACKUP_NOT_FOUND"
	ErrorCodeTampered          = "BACKUP_VERIFICATION_FAILED"
	ErrorCodeAuthorization     = "BACKUP_RESTORE_CONFIRMATION_INVALID"
	ErrorCodeRestorePending    = "BACKUP_RESTORE_PENDING"
	ErrorCodeRecoveryConflict  = "BACKUP_RECOVERY_CONFLICT"
	ErrorCodeRestoreApply      = "BACKUP_RESTORE_APPLY_FAILED"
	ErrorCodeAutomaticDegraded = "BACKUP_DEGRADED"
)

func APIErrorFrom(err error) *APIError {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrValidation):
		return &APIError{Code: ErrorCodeValidation, Message: "バックアップの指定が正しくありません。"}
	case errors.Is(err, ErrNotFound):
		return &APIError{Code: ErrorCodeNotFound, Message: "指定したバックアップが見つかりません。"}
	case errors.Is(err, ErrTampered):
		return &APIError{Code: ErrorCodeTampered, Message: "バックアップの完全性を検証できないため、復元できません。"}
	case errors.Is(err, ErrRestoreAuthorization):
		return &APIError{Code: ErrorCodeAuthorization, Message: "復元の確認が期限切れです。もう一度プレビューしてください。"}
	case errors.Is(err, ErrRestorePending):
		return &APIError{Code: ErrorCodeRestorePending, Message: "復元が次回起動時に適用される状態です。先にアプリを再起動してください。"}
	case errors.Is(err, ErrRecoveryConflict):
		return &APIError{Code: ErrorCodeRecoveryConflict, Message: "別の復旧処理が次回起動を待っています。先にアプリを再起動してください。"}
	case errors.Is(err, ErrRestoreApply):
		return &APIError{Code: ErrorCodeRestoreApply, Message: "バックアップを適用できませんでした。現在のデータは変更していません。"}
	case errors.Is(err, ErrAutomaticDegraded):
		return &APIError{Code: ErrorCodeAutomaticDegraded, Message: "復旧が未完了のため、自動バックアップを作成できません。"}
	default:
		return &APIError{Code: ErrorCodeUnavailable, Message: "バックアップを利用できませんでした。既存のデータは変更していません。"}
	}
}
