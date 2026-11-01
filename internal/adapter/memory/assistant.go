package memory

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func usageKey(workspaceID id.ID, date string) string { return workspaceID.String() + ":" + date }

func (s *Store) GetAssistantSettings(_ context.Context, workspaceID id.ID) (assistant.Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.aiSettings[workspaceID]
	if !ok {
		return assistant.Settings{}, errx.NotFound("ai_settings_not_found", "assistant is not configured")
	}
	return v, nil
}

func (s *Store) SaveAssistantSettings(_ context.Context, v assistant.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.aiSettings[v.WorkspaceID]
	s.aiSettings[v.WorkspaceID] = v
	if err := s.flushAILocked(); err != nil {
		if had {
			s.aiSettings[v.WorkspaceID] = prev
		} else {
			delete(s.aiSettings, v.WorkspaceID)
		}
		return err
	}
	return nil
}

func (s *Store) DeleteAssistantSettings(_ context.Context, workspaceID id.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.aiSettings, workspaceID)
	return s.flushAILocked()
}

func (s *Store) ReserveAIUsage(_ context.Context, workspaceID id.ID, date string, limit int) (int, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := usageKey(workspaceID, date)
	if s.aiUsage[k] >= limit {
		return s.aiUsage[k], false, nil
	}
	s.aiUsage[k]++
	return s.aiUsage[k], true, nil
}

func (s *Store) ReleaseAIUsage(_ context.Context, workspaceID id.ID, date string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := usageKey(workspaceID, date)
	if s.aiUsage[k] > 0 {
		s.aiUsage[k]--
	}
	return nil
}

func (s *Store) AIUsage(_ context.Context, workspaceID id.ID, date string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aiUsage[usageKey(workspaceID, date)], nil
}
