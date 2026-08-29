package noteexport

import (
	"context"
	"errors"
	"path/filepath"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/note"
)

type NoteReader interface {
	Get(context.Context, string) (note.Note, error)
}

type NoteProtectionReader interface {
	NoteLockStatus(context.Context, string) (protected bool, locked bool, source string, err error)
}

type Service struct {
	reader           NoteReader
	protectionReader NoteProtectionReader
}

func NewService(reader NoteReader, protectionReader NoteProtectionReader) *Service {
	return &Service{reader: reader, protectionReader: protectionReader}
}

func (service *Service) Export(ctx context.Context, path string, input Input) (Result, error) {
	if validationError := ValidateInput(input); validationError != nil {
		return Result{Error: validationError}, nil
	}
	finalPath, pathError := outputPath(path, input.Format)
	if pathError != nil {
		return Result{Error: pathError}, nil
	}
	if service == nil || service.reader == nil {
		return NewErrorResult(ErrorCodeUnavailable, "エクスポート機能を利用できません。", "", true), nil
	}
	if err := ctx.Err(); err != nil {
		return NewErrorResult(ErrorCodeUnavailable, "エクスポートを完了できませんでした。", "", true), nil
	}

	current, err := service.reader.Get(ctx, input.NoteID)
	if err != nil {
		return resultFromReadError(err), nil
	}
	protected := current.Protected
	locked := current.Locked
	if service.protectionReader != nil {
		protected, locked, _, err = service.protectionReader.NoteLockStatus(ctx, input.NoteID)
		if err != nil {
			return NewErrorResult(ErrorCodeUnavailable, "ノートの保護状態を確認できませんでした。", "", true), nil
		}
	}
	if locked {
		return NewErrorResult(ErrorCodeLocked, "ノートのロックを解除してからエクスポートしてください。", "", false), nil
	}
	if current.Revision != input.ExpectedRevision || current.Content != input.Markdown {
		return NewErrorResult(ErrorCodeStale, "ノートが更新されています。最新の内容を保存してから再試行してください。", "", true), nil
	}
	if protected && !input.AllowPlaintextProtected {
		return NewErrorResult(
			ErrorCodeProtectedConfirmationRequired,
			"保護されたノートは暗号化領域外へ平文で保存されます。確認してから再試行してください。",
			"allowPlaintextProtected",
			false,
		), nil
	}

	var content []byte
	switch input.Format {
	case FormatHTML:
		content, err = renderHTMLDocument(current.Title, input.HTMLFragment)
		if err != nil {
			return NewErrorResult(ErrorCodeRenderFailed, "HTMLを生成できませんでした。", "htmlFragment", false), nil
		}
	case FormatPDF:
		var pdfError *APIError
		content, pdfError = decodePDF(input.PDFBase64)
		if pdfError != nil {
			return Result{Error: pdfError}, nil
		}
	default:
		return NewErrorResult(ErrorCodeInvalidFormat, "エクスポート形式を確認できません。", "format", false), nil
	}

	if err := ctx.Err(); err != nil {
		return NewErrorResult(ErrorCodeUnavailable, "エクスポートを完了できませんでした。", "", true), nil
	}
	if err := writeFileAtomic(finalPath, content); err != nil {
		return NewErrorResult(ErrorCodeWriteFailed, "ファイルへ書き込めませんでした。保存先を確認して再試行してください。", "", true), nil
	}
	return Result{ExportedName: filepath.Base(finalPath)}, nil
}

func resultFromReadError(err error) Result {
	switch {
	case errors.Is(err, note.ErrNotFound):
		return NewErrorResult(ErrorCodeNoteNotFound, "エクスポートするノートが見つかりません。", "", false)
	case errors.Is(err, contentlock.ErrLocked):
		return NewErrorResult(ErrorCodeLocked, "ノートのロックを解除してからエクスポートしてください。", "", false)
	default:
		return NewErrorResult(ErrorCodeUnavailable, "ノートを読み込めませんでした。", "", true)
	}
}
