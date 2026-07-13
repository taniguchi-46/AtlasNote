package note

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const searchSnippetLimit = 240

// SearchDocument is the Markdown snapshot stored in the derived FTS index.
// Revision and ContentHash let reconciliation determine whether the snapshot
// still represents the canonical note and Markdown file.
type SearchDocument struct {
	NoteID      string
	Title       string
	Body        string
	Revision    int64
	ContentHash string
	IndexedAt   time.Time
}

// UpsertSearchIndex replaces one note's indexed Markdown snapshot and its
// reconciliation state atomically.
func (r *Repository) UpsertSearchIndex(ctx context.Context, document SearchDocument) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin search index upsert tx: %w", err)
	}
	defer tx.Rollback()

	if err := upsertSearchDocument(ctx, tx, document); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit search index upsert tx: %w", err)
	}

	return nil
}

// DeleteSearchIndex removes one note from the derived FTS index.
func (r *Repository) DeleteSearchIndex(ctx context.Context, noteID string) error {
	if noteID == "" {
		return fmt.Errorf("delete search index: note ID is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin search index delete tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM note_search WHERE note_id = ?", noteID); err != nil {
		return fmt.Errorf("delete search index document: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM note_search_state WHERE note_id = ?", noteID); err != nil {
		return fmt.Errorf("delete search index state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit search index delete tx: %w", err)
	}

	return nil
}

// RefreshSearchIndexState advances the reconciliation state when note
// metadata changes without changing the indexed title or Markdown body.
func (r *Repository) RefreshSearchIndexState(ctx context.Context, noteID string, revision int64, contentHash string) error {
	if noteID == "" {
		return fmt.Errorf("refresh search index state: note ID is required")
	}
	if revision < 1 {
		return fmt.Errorf("refresh search index state: revision must be at least 1")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin search index state refresh tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
UPDATE note_search_state
SET indexed_revision = ?, content_hash = ?, indexed_at = ?
WHERE note_id = ?
`, revision, contentHash, formatTime(time.Now().UTC()), noteID)
	if err != nil {
		return fmt.Errorf("refresh search index state: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read search index state refresh result: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("refresh search index state: note ID is not indexed")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit search index state refresh tx: %w", err)
	}
	return nil
}

// ReplaceSearchIndex rebuilds the complete derived index in one transaction.
// The caller is responsible for obtaining canonical note/Markdown snapshots.
func (r *Repository) ReplaceSearchIndex(ctx context.Context, documents []SearchDocument) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin search index rebuild tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM note_search"); err != nil {
		return fmt.Errorf("clear search index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM note_search_state"); err != nil {
		return fmt.Errorf("clear search index state: %w", err)
	}

	for _, document := range documents {
		if err := upsertSearchDocument(ctx, tx, document); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit search index rebuild tx: %w", err)
	}

	return nil
}

func upsertSearchDocument(ctx context.Context, executor sqlExecutor, document SearchDocument) error {
	if document.NoteID == "" {
		return fmt.Errorf("upsert search index: note ID is required")
	}
	if document.Revision < 1 {
		return fmt.Errorf("upsert search index: revision must be at least 1")
	}
	if document.IndexedAt.IsZero() {
		document.IndexedAt = time.Now().UTC()
	}

	if _, err := executor.ExecContext(ctx, "DELETE FROM note_search WHERE note_id = ?", document.NoteID); err != nil {
		return fmt.Errorf("replace search index document: %w", err)
	}
	if _, err := executor.ExecContext(
		ctx,
		"INSERT INTO note_search(note_id, title, body) VALUES (?, ?, ?)",
		document.NoteID,
		document.Title,
		document.Body,
	); err != nil {
		return fmt.Errorf("insert search index document: %w", err)
	}
	if _, err := executor.ExecContext(ctx, `
INSERT INTO note_search_state(note_id, indexed_revision, content_hash, indexed_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(note_id) DO UPDATE SET
    indexed_revision = excluded.indexed_revision,
    content_hash = excluded.content_hash,
    indexed_at = excluded.indexed_at
`, document.NoteID, document.Revision, document.ContentHash, formatTime(document.IndexedAt)); err != nil {
		return fmt.Errorf("upsert search index state: %w", err)
	}

	return nil
}

// Search executes the validated search contract against the derived FTS
// index. Validation and known index-state failures are returned in the
// structured result; unexpected database errors are returned as Go errors.
func (r *Repository) Search(ctx context.Context, input SearchInput) (SearchResult, error) {
	normalized, validationError := normalizeSearchInput(input)
	result := SearchResult{
		Items:    make([]SearchItem, 0),
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
	}
	if validationError != nil {
		result.Error = validationError
		return result, nil
	}
	if normalized.Query == "" {
		return result, nil
	}

	whereSQL, whereArgs, useFTS := buildSearchWhere(normalized)
	fromSQL := " FROM notes"
	if normalized.Scope == SearchScopeAll {
		fromSQL = " FROM note_search JOIN notes ON notes.id = note_search.note_id"
	}

	countQuery := "SELECT COUNT(*)" + fromSQL + whereSQL
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		if isSearchIndexUnavailable(err) {
			result.Error = searchIndexError(SearchErrorIndexNotReady)
			return result, nil
		}
		return result, fmt.Errorf("count note search results: %w", err)
	}

	result.Total = total
	result.HasNext = normalized.Page*normalized.PageSize < total
	if total == 0 {
		return result, nil
	}

	selectColumns := `
SELECT notes.id, notes.notebook_id, notes.title,
       notes.is_favorite, notes.is_pinned, notes.is_trashed,
       notes.revision, notes.created_at, notes.updated_at`
	if useFTS {
		selectColumns += ", snippet(note_search, -1, '<mark>', '</mark>', '…', 32)"
	} else {
		selectColumns += ", ''"
	}
	orderBy := " ORDER BY notes.updated_at DESC, notes.id ASC"
	if useFTS {
		orderBy = " ORDER BY bm25(note_search) ASC, notes.updated_at DESC, notes.id ASC"
	}
	query := selectColumns + fromSQL + whereSQL + orderBy + " LIMIT ? OFFSET ?"
	args := append(append([]any(nil), whereArgs...), normalized.PageSize, (normalized.Page-1)*normalized.PageSize)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		if isSearchIndexUnavailable(err) {
			result.Error = searchIndexError(SearchErrorIndexNotReady)
			return result, nil
		}
		return result, fmt.Errorf("search notes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item SearchItem
		var createdAt string
		var updatedAt string
		var snippet sql.NullString
		if err := rows.Scan(
			&item.Note.ID,
			&item.Note.NotebookID,
			&item.Note.Title,
			&item.Note.IsFavorite,
			&item.Note.IsPinned,
			&item.Note.IsTrashed,
			&item.Note.Revision,
			&createdAt,
			&updatedAt,
			&snippet,
		); err != nil {
			return result, fmt.Errorf("scan note search result: %w", err)
		}

		item.Note.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return result, err
		}
		item.Note.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return result, err
		}
		item.Snippet = truncateSearchSnippet(snippet.String)
		if normalized.Scope == SearchScopeTitle {
			item.MatchScope = SearchScopeTitle
		} else {
			item.MatchScope = "both"
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate note search results: %w", err)
	}

	return result, nil
}

type normalizedSearchInput struct {
	Query          string
	Scope          string
	NotebookID     *string
	IncludeTrashed bool
	Page           int
	PageSize       int
}

func normalizeSearchInput(input SearchInput) (normalizedSearchInput, *SearchError) {
	normalized := normalizedSearchInput{
		Query:          strings.TrimSpace(input.Query),
		Scope:          input.Scope,
		NotebookID:     input.NotebookID,
		IncludeTrashed: input.IncludeTrashed,
		Page:           input.Page,
		PageSize:       input.PageSize,
	}
	if normalized.Scope == "" {
		normalized.Scope = SearchScopeAll
	}
	if normalized.Page == 0 {
		normalized.Page = DefaultSearchPage
	}
	if normalized.PageSize == 0 {
		normalized.PageSize = DefaultSearchPageSize
	}

	if !utf8.ValidString(normalized.Query) {
		return normalized, &SearchError{
			Code:      SearchErrorQueryInvalid,
			Message:   "検索語に使用できない文字が含まれています。",
			Field:     "query",
			Retryable: false,
		}
	}
	if utf8.RuneCountInString(normalized.Query) > MaxSearchQueryLength {
		return normalized, &SearchError{
			Code:      SearchErrorQueryTooLong,
			Message:   "検索語が長すぎます。",
			Field:     "query",
			Retryable: false,
		}
	}
	for _, character := range normalized.Query {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return normalized, &SearchError{
				Code:      SearchErrorQueryInvalid,
				Message:   "検索語に使用できない文字が含まれています。",
				Field:     "query",
				Retryable: false,
			}
		}
	}
	if normalized.Scope != SearchScopeAll && normalized.Scope != SearchScopeTitle {
		return normalized, &SearchError{
			Code:      SearchErrorScopeInvalid,
			Message:   "検索範囲が不正です。",
			Field:     "scope",
			Retryable: false,
		}
	}
	if normalized.Page < 1 || normalized.Page > MaxSearchPage {
		return normalized, &SearchError{
			Code:      SearchErrorPageInvalid,
			Message:   "ページ番号が不正です。",
			Field:     "page",
			Retryable: false,
		}
	}
	if normalized.PageSize < 1 || normalized.PageSize > MaxSearchPageSize {
		return normalized, &SearchError{
			Code:      SearchErrorPageSizeInvalid,
			Message:   "ページサイズが不正です。",
			Field:     "pageSize",
			Retryable: false,
		}
	}

	return normalized, nil
}

func buildSearchWhere(input normalizedSearchInput) (string, []any, bool) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)
	useFTS := false

	if input.Scope == SearchScopeTitle {
		conditions = append(conditions, "notes.title LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLikePattern(input.Query)+"%")
	} else {
		terms := strings.Fields(input.Query)
		useFTS = len(terms) > 0 && allSearchTermsSupportFTS(terms)
		if useFTS {
			conditions = append(conditions, "note_search MATCH ?")
			args = append(args, buildFTSQuery(terms))
		} else {
			for _, term := range terms {
				pattern := "%" + escapeLikePattern(term) + "%"
				conditions = append(conditions, "(note_search.title LIKE ? ESCAPE '\\' OR note_search.body LIKE ? ESCAPE '\\')")
				args = append(args, pattern, pattern)
			}
		}
	}

	if !input.IncludeTrashed {
		conditions = append(conditions, "notes.is_trashed = 0")
	}
	if input.NotebookID != nil {
		conditions = append(conditions, "notes.notebook_id = ?")
		args = append(args, *input.NotebookID)
	}

	if len(conditions) == 0 {
		return "", args, useFTS
	}
	return " WHERE " + strings.Join(conditions, " AND "), args, useFTS
}

func allSearchTermsSupportFTS(terms []string) bool {
	for _, term := range terms {
		if utf8.RuneCountInString(term) < 3 {
			return false
		}
	}
	return true
}

func buildFTSQuery(terms []string) string {
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " AND ")
}

func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "%", `\%`)
	return strings.ReplaceAll(value, "_", `\_`)
}

func truncateSearchSnippet(value string) string {
	runes := []rune(value)
	if len(runes) <= searchSnippetLimit {
		return value
	}
	return string(runes[:searchSnippetLimit-1]) + "…"
}

func isSearchIndexUnavailable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table: note_search")
}

func searchIndexError(code string) *SearchError {
	message := "検索索引を利用できません。"
	if code == SearchErrorIndexInconsistent {
		message = "検索索引を更新しています。"
	}
	return &SearchError{Code: code, Message: message, Retryable: true}
}
