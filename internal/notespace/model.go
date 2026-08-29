package notespace

import (
	"errors"

	"atlasnote/internal/datalock"
)

const (
	ErrorCodeUnavailable    = "STORAGE_SPACE_UNAVAILABLE"
	ErrorCodeCatalogInvalid = "STORAGE_SPACE_CATALOG_INVALID"
	ErrorCodeNameInvalid    = "STORAGE_SPACE_NAME_INVALID"
	ErrorCodeNameConflict   = "STORAGE_SPACE_NAME_CONFLICT"
	ErrorCodeNotFound       = "STORAGE_SPACE_NOT_FOUND"
	ErrorCodeInUse          = "STORAGE_SPACE_IN_USE"
	ErrorCodePathInvalid    = "STORAGE_SPACE_PATH_INVALID"
)

var (
	ErrCatalogInvalid = errors.New("storage space catalog is invalid")
	ErrNameInvalid    = errors.New("storage space name is invalid")
	ErrNameConflict   = errors.New("storage space name already exists")
	ErrNotFound       = errors.New("storage space was not found")
	ErrUnsafePath     = errors.New("storage space path is unsafe")
	ErrUnavailable    = errors.New("storage space is unavailable")
)

type Space struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	Legacy    bool   `json:"legacy"`
	CreatedAt string `json:"createdAt"`
}

type CreateInput struct {
	Name string `json:"name"`
}

type SelectInput struct {
	ID string `json:"id"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ListResult struct {
	Spaces        []Space   `json:"spaces"`
	ActiveSpaceID string    `json:"activeSpaceId"`
	Error         *APIError `json:"error,omitempty"`
}

type MutationResult struct {
	Space           *Space    `json:"space,omitempty"`
	ActiveSpaceID   string    `json:"activeSpaceId,omitempty"`
	RestartRequired bool      `json:"restartRequired"`
	Error           *APIError `json:"error,omitempty"`
}

func APIErrorFrom(err error) *APIError {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, ErrNameInvalid):
		return &APIError{Code: ErrorCodeNameInvalid, Message: "保存空間の名前は1〜80文字で入力してください。"}
	case errors.Is(err, ErrNameConflict):
		return &APIError{Code: ErrorCodeNameConflict, Message: "同じ名前の保存空間がすでにあります。"}
	case errors.Is(err, ErrNotFound):
		return &APIError{Code: ErrorCodeNotFound, Message: "選択した保存空間が見つかりません。"}
	case errors.Is(err, datalock.ErrAlreadyLocked):
		return &APIError{Code: ErrorCodeInUse, Message: "選択した保存空間は別のAtlas Noteで使用中です。"}
	case errors.Is(err, ErrCatalogInvalid):
		return &APIError{Code: ErrorCodeCatalogInvalid, Message: "保存空間の管理情報を検証できませんでした。管理情報は変更していません。"}
	case errors.Is(err, ErrUnsafePath):
		return &APIError{Code: ErrorCodePathInvalid, Message: "保存空間の内部パスを安全に検証できませんでした。"}
	default:
		return &APIError{Code: ErrorCodeUnavailable, Message: "保存空間を利用できませんでした。データは変更していません。"}
	}
}
