package ai

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"atlasnote/internal/note"
)

const (
	aiMaxContextSources   = 10
	aiMaxExplicitNoteIDs  = 10
	aiContextBytes        = 48 * 1024
	aiContextNoteBytes    = 16 * 1024
	aiMaxSearchQueryBytes = 2 * 1024
)

type ContextNote struct {
	NoteID           string
	Title            string
	Content          string
	Revision         int64
	Snippet          string
	IsTrashed        bool
	CharacterCount   int
	ContentByte      int
	TotalContentByte int
	ContentTruncated bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type NoteContextProvider interface {
	Get(ctx context.Context, noteID string) (ContextNote, error)
	Search(ctx context.Context, query string, limit int) ([]ContextNote, error)
	ListBacklinks(ctx context.Context, noteID string, limit int) ([]ContextNote, error)
}

type noteServiceContextProvider struct {
	service *note.Service
}

func NewNoteContextProvider(service *note.Service) NoteContextProvider {
	if service == nil {
		return nil
	}
	return noteServiceContextProvider{service: service}
}

func (p noteServiceContextProvider) Get(ctx context.Context, noteID string) (ContextNote, error) {
	current, err := p.service.Get(ctx, noteID)
	if err != nil {
		return ContextNote{}, err
	}
	return ContextNote{
		NoteID:    current.ID,
		Title:     current.Title,
		Content:   current.Content,
		Revision:  current.Revision,
		IsTrashed: current.IsTrashed,
		CreatedAt: current.CreatedAt,
		UpdatedAt: current.UpdatedAt,
	}, nil
}

func (p noteServiceContextProvider) Search(ctx context.Context, query string, limit int) ([]ContextNote, error) {
	result, err := p.service.Search(ctx, note.SearchInput{
		Query:          query,
		Scope:          note.SearchScopeAll,
		IncludeTrashed: false,
		Page:           1,
		PageSize:       limit,
	})
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, errors.New("AI context search index is unavailable")
	}

	items := make([]ContextNote, 0, len(result.Items))
	for _, item := range result.Items {
		current, err := p.Get(ctx, item.Note.ID)
		if err != nil {
			return nil, err
		}
		current.Snippet = item.Snippet
		items = append(items, current)
	}
	return items, nil
}

func (p noteServiceContextProvider) ListBacklinks(ctx context.Context, noteID string, limit int) ([]ContextNote, error) {
	result, err := p.service.ListBacklinks(ctx, note.BacklinkListInput{
		NoteID:   noteID,
		Page:     1,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}

	items := make([]ContextNote, 0, len(result.Items))
	for _, item := range result.Items {
		current, err := p.Get(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, current)
	}
	return items, nil
}

func (s *Service) PrepareContext(ctx context.Context, input AIContextInput) ([]AIContextSource, error) {
	normalized, err := normalizeContextInput(input)
	if err != nil {
		return nil, err
	}
	contextNotes, err := s.collectContext(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return contextSources(contextNotes), nil
}

func (s *Service) collectContext(ctx context.Context, input AIContextInput) ([]ContextNote, error) {
	provider := s.getContextProvider()
	if provider == nil {
		if len(input.NoteIDs) == 0 && strings.TrimSpace(input.SearchQuery) == "" {
			return []ContextNote{}, nil
		}
		return nil, ErrProviderUnavailable
	}
	normalized, err := normalizeContextInput(input)
	if err != nil {
		return nil, err
	}

	items := make([]ContextNote, 0, aiMaxContextSources)
	seen := make(map[string]struct{})
	add := func(item ContextNote) bool {
		if len(items) >= aiMaxContextSources || item.NoteID == "" || item.IsTrashed {
			return false
		}
		if _, ok := seen[item.NoteID]; ok {
			return false
		}
		remaining := aiContextBytes
		for _, existing := range items {
			remaining -= len([]byte(existing.Content))
		}
		if remaining <= 0 {
			return false
		}
		item = populateContextMetrics(item)
		fullContentByte := item.TotalContentByte
		item.Content = limitUTF8Bytes(item.Content, minInt(remaining, aiContextNoteBytes))
		if strings.TrimSpace(item.Content) == "" {
			return false
		}
		item.ContentByte = len([]byte(item.Content))
		item.ContentTruncated = item.ContentByte < fullContentByte
		seen[item.NoteID] = struct{}{}
		items = append(items, item)
		return true
	}

	for _, noteID := range normalized.NoteIDs {
		item, err := provider.Get(ctx, noteID)
		if err != nil {
			return nil, ErrInputInvalid
		}
		if item.IsTrashed {
			return nil, ErrInputInvalid
		}
		if !add(item) {
			return nil, ErrInputTooLarge
		}
	}
	if normalized.SearchQuery != "" {
		searched, err := provider.Search(ctx, normalized.SearchQuery, aiMaxContextSources)
		if err != nil {
			return nil, ErrProviderUnavailable
		}
		for _, item := range searched {
			if !add(item) {
				break
			}
		}
	}
	if normalized.IncludeBacklinks && len(normalized.NoteIDs) > 0 {
		backlinks, err := provider.ListBacklinks(ctx, normalized.NoteIDs[0], aiMaxContextSources)
		if err != nil {
			return nil, ErrProviderUnavailable
		}
		for _, item := range backlinks {
			if !add(item) {
				break
			}
		}
	}
	return items, nil
}

func (s *Service) getContextProvider() NoteContextProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contextProvider
}

func normalizeContextInput(input AIContextInput) (AIContextInput, error) {
	seen := make(map[string]struct{})
	noteIDs := make([]string, 0, len(input.NoteIDs))
	for _, value := range input.NoteIDs {
		noteID := strings.TrimSpace(value)
		if noteID == "" {
			continue
		}
		if _, ok := seen[noteID]; ok {
			continue
		}
		seen[noteID] = struct{}{}
		noteIDs = append(noteIDs, noteID)
	}
	if len(noteIDs) > aiMaxExplicitNoteIDs {
		return AIContextInput{}, ErrInputTooLarge
	}
	searchQuery := strings.TrimSpace(input.SearchQuery)
	if len([]byte(searchQuery)) > aiMaxSearchQueryBytes {
		return AIContextInput{}, ErrInputTooLarge
	}
	return AIContextInput{NoteIDs: noteIDs, SearchQuery: searchQuery, IncludeBacklinks: input.IncludeBacklinks}, nil
}

func normalizeSources(sources []AIHistorySource) ([]AIHistorySource, error) {
	if len(sources) > aiMaxContextSources {
		return nil, ErrInputTooLarge
	}
	seen := make(map[string]struct{}, len(sources))
	result := make([]AIHistorySource, 0, len(sources))
	for _, source := range sources {
		noteID := strings.TrimSpace(source.NoteID)
		if noteID == "" || source.InputRevision < 1 {
			return nil, ErrInputInvalid
		}
		if _, ok := seen[noteID]; ok {
			return nil, ErrInputInvalid
		}
		seen[noteID] = struct{}{}
		result = append(result, AIHistorySource{NoteID: noteID, InputRevision: source.InputRevision})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].NoteID < result[j].NoteID })
	return result, nil
}

// validateExpectedSources ensures that the snapshots shown in the UI are the
// same snapshots sent to the provider. Empty expected sources preserve the
// existing optional-context behavior for callers that did not preview first.
func validateExpectedSources(expected []AIHistorySource, actual []ContextNote) error {
	if len(expected) == 0 {
		return nil
	}
	normalizedExpected, err := normalizeSources(expected)
	if err != nil {
		return err
	}
	actualSources := make([]AIHistorySource, 0, len(actual))
	for _, item := range actual {
		actualSources = append(actualSources, AIHistorySource{
			NoteID:        item.NoteID,
			InputRevision: item.Revision,
		})
	}
	normalizedActual, err := normalizeSources(actualSources)
	if err != nil {
		return err
	}
	if len(normalizedExpected) != len(normalizedActual) {
		return ErrContextChanged
	}
	for index := range normalizedExpected {
		if normalizedExpected[index] != normalizedActual[index] {
			return ErrContextChanged
		}
	}
	return nil
}

func contextSources(items []ContextNote) []AIContextSource {
	result := make([]AIContextSource, 0, len(items))
	for _, item := range items {
		item = populateContextMetrics(item)
		result = append(result, AIContextSource{
			NoteID:           item.NoteID,
			Title:            item.Title,
			Revision:         item.Revision,
			Snippet:          item.Snippet,
			CharacterCount:   item.CharacterCount,
			ContentByte:      item.ContentByte,
			TotalContentByte: item.TotalContentByte,
			ContentTruncated: item.ContentTruncated,
			CreatedAt:        item.CreatedAt,
			UpdatedAt:        item.UpdatedAt,
		})
	}
	return result
}

func populateContextMetrics(item ContextNote) ContextNote {
	if item.TotalContentByte == 0 && item.Content != "" {
		item.TotalContentByte = len([]byte(item.Content))
	}
	if item.CharacterCount == 0 && item.Content != "" {
		item.CharacterCount = utf16CharacterCount(item.Content)
	}
	if item.ContentByte == 0 && item.Content != "" {
		item.ContentByte = len([]byte(item.Content))
	}
	if item.TotalContentByte > 0 && item.ContentByte < item.TotalContentByte {
		item.ContentTruncated = true
	}
	return item
}

// utf16CharacterCount matches the browser's String.length semantics used by
// the note editor, so the AI metadata and the visible editor count agree for
// Japanese text, emoji, and other supplementary-plane characters.
func utf16CharacterCount(value string) int {
	return len(utf16.Encode([]rune(value)))
}

func limitUTF8Bytes(value string, limit int) string {
	if limit <= 0 || len([]byte(value)) <= limit {
		return value
	}
	bytes := []byte(value[:0])
	for _, runeValue := range value {
		candidate := string(runeValue)
		if len(bytes)+len([]byte(candidate)) > limit {
			break
		}
		bytes = append(bytes, []byte(candidate)...)
	}
	return string(bytes)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
