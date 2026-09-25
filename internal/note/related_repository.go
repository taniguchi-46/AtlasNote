package note

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

const relatedCandidateLimit = 80

// relatedSignals contains only indexed metadata. No Markdown body is loaded
// while gathering candidates.
type relatedSignals struct {
	outbound bool
	backlink bool
	tags     []string
}

func relatedScope(allowed map[string]bool) (string, []any) {
	if allowed == nil {
		return "", nil
	}
	ids := make([]string, 0, len(allowed))
	for id := range allowed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	return " AND notes.notebook_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")", args
}

func (r *Repository) relatedLinks(ctx context.Context, noteID string, outbound bool, allowed map[string]bool) ([]string, error) {
	column, filter := "target_note_id", "source_note_id"
	if !outbound {
		column, filter = "source_note_id", "target_note_id"
	}
	scope, scopeArgs := relatedScope(allowed)
	reverseSource, reverseTarget := column, filter
	if !outbound {
		reverseSource, reverseTarget = filter, column
	}
	query := fmt.Sprintf(`SELECT note_links.%s FROM note_links JOIN notes ON notes.id = note_links.%s WHERE note_links.%s = ? AND notes.is_trashed = 0%s ORDER BY EXISTS (SELECT 1 FROM note_links AS reverse WHERE reverse.source_note_id = note_links.%s AND reverse.target_note_id = note_links.%s) DESC, notes.updated_at DESC, notes.id LIMIT ?`, column, column, filter, scope, reverseSource, reverseTarget)
	args := append([]any{noteID}, scopeArgs...)
	rows, err := r.db.QueryContext(ctx, query, append(args, relatedCandidateLimit)...)
	if err != nil {
		return nil, fmt.Errorf("list related links: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan related link: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) relatedTags(ctx context.Context, noteID string, allowed map[string]bool) (map[string][]string, error) {
	scope, scopeArgs := relatedScope(allowed)
	query := `
WITH candidates AS (
 SELECT other.note_id, COUNT(*) AS shared_count
 FROM note_tags AS source
 JOIN note_tags AS other ON other.tag_id = source.tag_id AND other.note_id <> source.note_id
 JOIN notes ON notes.id = other.note_id
 WHERE source.note_id = ? AND notes.is_trashed = 0` + scope + `
 GROUP BY other.note_id ORDER BY shared_count DESC, other.note_id LIMIT ?
)
SELECT candidates.note_id, tags.name
FROM candidates
JOIN note_tags AS source ON source.note_id = ?
JOIN note_tags AS other ON other.tag_id = source.tag_id AND other.note_id = candidates.note_id
JOIN tags ON tags.id = source.tag_id
ORDER BY candidates.shared_count DESC, candidates.note_id, tags.name`
	args := append([]any{noteID}, scopeArgs...)
	args = append(args, relatedCandidateLimit, noteID)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list related tags: %w", err)
	}
	defer rows.Close()
	result := make(map[string][]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan related tag: %w", err)
		}
		if len(result[id]) < 2 {
			result[id] = append(result[id], name)
		}
	}
	return result, rows.Err()
}

func (r *Repository) relatedTextMatches(ctx context.Context, sourceID, term string, allowed map[string]bool) (map[string]string, error) {
	input := normalizedSearchInput{Query: term, Scope: SearchScopeAll}
	where, whereArgs, useFTS := buildSearchWhere(input)
	scope, scopeArgs := relatedScope(allowed)
	evidence, evidenceArgs := searchEvidenceColumns(strings.Fields(term), useFTS)
	order := " ORDER BY 3 DESC, notes.updated_at DESC, notes.id"
	if useFTS {
		order = " ORDER BY 3 DESC, bm25(note_search), notes.id"
	}
	query := `SELECT note_search.note_id, ` + evidence + ` FROM note_search
JOIN notes ON notes.id = note_search.note_id` + where + ` AND notes.id <> ?` + scope + order + ` LIMIT ?`
	args := append(evidenceArgs, whereArgs...)
	args = append(args, sourceID)
	args = append(args, scopeArgs...)
	args = append(args, relatedCandidateLimit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list related text matches: %w", err)
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var id, snippet string
		var titleHit, bodyHit bool
		if err := rows.Scan(&id, &snippet, &titleHit, &bodyHit); err != nil {
			return nil, fmt.Errorf("scan related text match: %w", err)
		}
		switch {
		case titleHit && bodyHit:
			result[id] = "both"
		case titleHit:
			result[id] = SearchScopeTitle
		case bodyHit:
			result[id] = "body"
		}
	}
	return result, rows.Err()
}

// relatedSnippet reads a bounded prefix from the derived index and verifies
// that it still represents the current canonical revision.
func (r *Repository) relatedSnippet(ctx context.Context, noteID string, revision int64) (string, error) {
	var snippet sql.NullString
	var indexedRevision int64
	err := r.db.QueryRowContext(ctx, `
SELECT substr(note_search.body, 1, 160), note_search_state.indexed_revision
FROM note_search
JOIN note_search_state ON note_search_state.note_id = note_search.note_id
WHERE note_search.note_id = ?`, noteID).Scan(&snippet, &indexedRevision)
	if err != nil {
		return "", fmt.Errorf("read related index: %w", err)
	}
	if indexedRevision != revision {
		return "", fmt.Errorf("related search index is inconsistent")
	}
	return truncateSearchSnippet(snippet.String), nil
}
