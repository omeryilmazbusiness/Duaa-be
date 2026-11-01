package memory

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) CreateDecision(_ context.Context, d executive.Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.Active() {
		for _, existing := range s.decisions {
			if existing.WorkspaceID == d.WorkspaceID && existing.SourceKey == d.SourceKey && existing.Active() {
				return errx.Conflict("decision_exists", "an open decision already exists for this source")
			}
		}
	}
	s.decisions[d.ID] = d
	return nil
}

func (s *Store) GetDecision(_ context.Context, did id.ID) (executive.Decision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.decisions[did]
	if !ok {
		return executive.Decision{}, errx.NotFound("decision_not_found", "decision not found")
	}
	return d, nil
}

func (s *Store) ListDecisions(_ context.Context, workspaceID id.ID) ([]executive.Decision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]executive.Decision, 0)
	for _, d := range s.decisions {
		if d.WorkspaceID == workspaceID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *Store) UpdateDecision(_ context.Context, d executive.Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.decisions[d.ID]; !ok {
		return errx.NotFound("decision_not_found", "decision not found")
	}
	s.decisions[d.ID] = d
	return nil
}
