package noteimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"atlasnote/internal/note"
)

const MaxSourceBytes int64 = 2 * 1024 * 1024

var (
	errSourceTooLarge = errors.New("import source is too large")
	firstLineHeading  = regexp.MustCompile(`^[ \t]{0,3}#{1,6}[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$`)
)

type noteWriter interface {
	Create(context.Context, note.CreateInput) (note.Note, error)
	CreateNotebook(context.Context, note.NotebookCreateInput) (note.Notebook, error)
}

type Service struct {
	notes      noteWriter
	readSource func(string) ([]byte, error)
	mu         sync.Mutex
}

func NewService(notes noteWriter) *Service {
	return &Service{
		notes:      notes,
		readSource: readSourceFile,
	}
}

// Import reads selected source files one by one. Conversion errors are kept
// per file; once persistence fails, the remaining files are not attempted so a
// storage failure cannot be amplified into a larger partial write.
func (s *Service) Import(ctx context.Context, paths []string, input Input) Result {
	if !s.mu.TryLock() {
		return NewErrorResult(ErrorCodeBusy, "別のインポート処理が実行中です。", true)
	}
	defer s.mu.Unlock()

	result := NewResult()
	destination, newNotebookName, destinationErr := validateDestination(input)
	if destinationErr != nil {
		result.Error = &APIError{
			Code:    ErrorCodeInvalidDestination,
			Message: "保存先の指定が正しくありません。",
		}
		return result
	}
	titleMode, titleModeErr := validateTitleMode(input.TitleMode)
	if titleModeErr != nil {
		result.Error = &APIError{
			Code:    ErrorCodeInvalidTitleMode,
			Message: "タイトルの決定方法が正しくありません。",
		}
		return result
	}

	createdNotebook := false
	for _, sourcePath := range paths {
		if err := ctx.Err(); err != nil {
			result.Error = &APIError{
				Code:      ErrorCodeCancelled,
				Message:   "インポート処理が中断されました。",
				Retryable: true,
			}
			return result
		}

		candidate, failure := s.readCandidate(sourcePath, titleMode)
		if failure != nil {
			result.Failures = append(result.Failures, *failure)
			continue
		}

		if newNotebookName != "" && !createdNotebook {
			created, err := s.notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: newNotebookName})
			if err != nil {
				result.Error = &APIError{
					Code:      ErrorCodeNotebookCreate,
					Message:   "新しいノートブックを作成できませんでした。",
					Retryable: true,
				}
				return result
			}
			destination = &created.ID
			createdNotebook = true
			result.CreatedNotebook = &CreatedNotebook{ID: created.ID, Name: created.Name}
		}

		created, err := s.notes.Create(ctx, note.CreateInput{
			NotebookID: destination,
			Title:      candidate.title,
			Content:    candidate.content,
		})
		if err != nil {
			result.Failures = append(result.Failures, FileFailure{
				SourceName: candidate.sourceName,
				Code:       FailureCodeCreate,
				Message:    "ノートを保存できませんでした。",
			})
			result.Error = &APIError{
				Code:      ErrorCodePersistence,
				Message:   "ノートの保存に失敗したため、残りのファイルは取り込みませんでした。",
				Retryable: true,
			}
			return result
		}

		result.Imported = append(result.Imported, ImportedNote{
			SourceName: candidate.sourceName,
			NoteID:     created.ID,
			Title:      created.Title,
		})
	}

	return result
}

type candidate struct {
	sourceName string
	title      string
	content    string
}

func (s *Service) readCandidate(sourcePath string, titleMode TitleMode) (candidate, *FileFailure) {
	sourceName := safeSourceName(sourcePath)
	extension := strings.ToLower(filepath.Ext(sourceName))
	if !isSupportedExtension(extension) {
		return candidate{}, &FileFailure{
			SourceName: sourceName,
			Code:       FailureCodeUnsupportedFile,
			Message:    "対応していないファイル形式です。",
		}
	}

	data, err := s.readSource(sourcePath)
	if err != nil {
		code := FailureCodeRead
		message := "ファイルを読み込めませんでした。"
		if errors.Is(err, errSourceTooLarge) {
			code = FailureCodeTooLarge
			message = "ファイルが大きすぎます。"
		}
		return candidate{}, &FileFailure{SourceName: sourceName, Code: code, Message: message}
	}
	if !utf8.Valid(data) {
		return candidate{}, &FileFailure{
			SourceName: sourceName,
			Code:       FailureCodeEncoding,
			Message:    "UTF-8として読み込めないファイルです。",
		}
	}

	content := strings.TrimPrefix(string(data), "\uFEFF")
	headingTitle := ""
	metadataTitle := ""
	if extension == ".html" || extension == ".htm" {
		converted, err := convertHTML(content)
		if err != nil {
			code := FailureCodeHTML
			message := "HTMLをノートへ変換できませんでした。"
			if errors.Is(err, errHTMLWithoutVisibleContent) {
				code = FailureCodeEmptyHTML
				message = "HTMLに取り込める本文がありません。"
			}
			return candidate{}, &FileFailure{SourceName: sourceName, Code: code, Message: message}
		}
		content = converted.Content
		headingTitle = converted.HeadingTitle
		metadataTitle = converted.MetadataTitle
	} else {
		headingTitle = titleFromFirstLine(content)
	}

	title := resolveTitle(titleMode, sourceName, headingTitle, metadataTitle)
	return candidate{sourceName: sourceName, title: title, content: content}, nil
}

func resolveTitle(titleMode TitleMode, sourceName string, headingTitle string, metadataTitle string) string {
	titleCandidate := ""
	switch titleMode {
	case TitleModeAuto:
		titleCandidate = headingTitle
		if titleCandidate == "" {
			titleCandidate = metadataTitle
		}
	case TitleModeHeading:
		titleCandidate = headingTitle
	case TitleModeMetadata:
		titleCandidate = metadataTitle
	case TitleModeFilename:
		// The filename fallback below is the requested title source.
	}

	title := normalizeTitle(titleCandidate)
	if title == "" {
		title = normalizeTitle(sourceTitleFallback(sourceName))
	}
	if title == "" {
		title = "インポートしたノート"
	}

	return title
}

func validateDestination(input Input) (*string, string, error) {
	newNotebookName := ""
	if input.NewNotebookName != nil {
		newNotebookName = strings.TrimSpace(*input.NewNotebookName)
		if newNotebookName == "" {
			return nil, "", fmt.Errorf("invalid import destination")
		}
	}
	if input.NotebookID != nil {
		id := strings.TrimSpace(*input.NotebookID)
		if id == "" || input.NewNotebookName != nil {
			return nil, "", fmt.Errorf("invalid import destination")
		}
		return &id, "", nil
	}
	return nil, newNotebookName, nil
}

func validateTitleMode(value TitleMode) (TitleMode, error) {
	switch value {
	case "", TitleModeAuto:
		return TitleModeAuto, nil
	case TitleModeFilename, TitleModeHeading, TitleModeMetadata:
		return value, nil
	default:
		return "", fmt.Errorf("invalid import title mode")
	}
}

func readSourceFile(sourcePath string) ([]byte, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxSourceBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxSourceBytes {
		return nil, errSourceTooLarge
	}
	return data, nil
}

func isSupportedExtension(extension string) bool {
	switch extension {
	case ".md", ".txt", ".html", ".htm":
		return true
	default:
		return false
	}
}

func safeSourceName(sourcePath string) string {
	name := strings.TrimSpace(filepath.Base(sourcePath))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "不明なファイル"
	}
	return name
}

func sourceTitleFallback(sourceName string) string {
	extension := filepath.Ext(sourceName)
	name := strings.TrimSpace(strings.TrimSuffix(sourceName, extension))
	if name == "" {
		return "インポートしたノート"
	}
	return name
}

func titleFromFirstLine(content string) string {
	firstLine, _, _ := strings.Cut(content, "\n")
	firstLine = strings.TrimSuffix(firstLine, "\r")
	matches := firstLineHeading.FindStringSubmatch(firstLine)
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func normalizeTitle(value string) string {
	normalized := strings.Join(strings.Fields(value), " ")
	runes := []rune(normalized)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return normalized
}
