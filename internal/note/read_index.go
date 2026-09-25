package note

import (
	"context"
	"fmt"
)

// ValidateSearchIndexSnapshot verifies that the derived search entry still
// represents the canonical Markdown snapshot for the requested revision.
// External read adapters use this before returning snippets.
func (s *Service) ValidateSearchIndexSnapshot(ctx context.Context, noteID string, revision int64) error {
	releaseContent := s.beginContentAccess(ctx)
	defer releaseContent()
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
	}
	record, err := s.repository.Get(ctx, noteID)
	if err != nil {
		return err
	}
	if record.Revision != revision || s.searchIndexFailed {
		return ErrIndexInconsistent
	}
	state, found, err := s.repository.GetSearchIndexState(ctx, noteID)
	if err != nil {
		return err
	}
	if !found || state.IndexedRevision != revision {
		return ErrIndexInconsistent
	}
	matches, err := s.store.ContentMatches(ctx, noteID, state.ContentHash)
	if err != nil {
		return err
	}
	if !matches {
		return ErrIndexInconsistent
	}
	return nil
}

// ValidateLinkIndexSnapshot verifies that a source note's backlink data was
// derived from its current canonical Markdown snapshot.
func (s *Service) ValidateLinkIndexSnapshot(ctx context.Context, noteID string, revision int64) error {
	releaseContent := s.beginContentAccess(ctx)
	defer releaseContent()
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
	}
	record, err := s.repository.Get(ctx, noteID)
	if err != nil {
		return err
	}
	if record.Revision != revision || s.noteLinkIndexFailed {
		return ErrIndexInconsistent
	}
	state, found, err := s.repository.GetNoteLinkIndexState(ctx, noteID)
	if err != nil {
		return err
	}
	if !found || state.IndexedRevision != revision {
		return ErrIndexInconsistent
	}
	matches, err := s.store.ContentMatches(ctx, noteID, state.ContentHash)
	if err != nil {
		return err
	}
	if !matches {
		return fmt.Errorf("%w: backlink snapshot", ErrIndexInconsistent)
	}
	return nil
}
