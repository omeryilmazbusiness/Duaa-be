package memory

import (
	"context"
	"sort"

	"github.com/dua-os/dua-os-be/internal/domain/inbox"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) GetConnection(_ context.Context, workspaceID id.ID) (inbox.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.inboxConns[workspaceID]
	if !ok {
		return inbox.Connection{}, inbox.ConnectionNotFoundError()
	}
	return c, nil
}

func (s *Store) SaveConnection(_ context.Context, c inbox.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inboxConns == nil {
		s.inboxConns = map[id.ID]inbox.Connection{}
	}
	s.inboxConns[c.WorkspaceID] = c
	return nil
}

func (s *Store) DeleteConnection(_ context.Context, workspaceID id.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inboxConns, workspaceID)
	return nil
}

func (s *Store) UpsertMessage(_ context.Context, m inbox.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inboxMsgs == nil {
		s.inboxMsgs = map[id.ID]inbox.Message{}
	}
	for idKey, existing := range s.inboxMsgs {
		if existing.WorkspaceID == m.WorkspaceID && existing.ExternalID == m.ExternalID {
			m.ID = existing.ID
			if m.CreatedAt.IsZero() {
				m.CreatedAt = existing.CreatedAt
			}
			s.inboxMsgs[idKey] = m
			return nil
		}
	}
	s.inboxMsgs[m.ID] = m
	return nil
}

func (s *Store) GetMessage(_ context.Context, workspaceID, messageID id.ID) (inbox.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.inboxMsgs[messageID]
	if !ok || m.WorkspaceID != workspaceID {
		return inbox.Message{}, inbox.MessageNotFoundError()
	}
	return m, nil
}

func (s *Store) ListMessages(_ context.Context, workspaceID id.ID, limit int) ([]inbox.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]inbox.Message, 0)
	for _, m := range s.inboxMsgs {
		if m.WorkspaceID == workspaceID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReceivedAt.After(out[j].ReceivedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) CountUnread(_ context.Context, workspaceID id.ID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, m := range s.inboxMsgs {
		if m.WorkspaceID == workspaceID && !m.IsRead && m.Folder == inbox.FolderInbox {
			n++
		}
	}
	return n, nil
}

func (s *Store) DeleteMessages(_ context.Context, workspaceID id.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, m := range s.inboxMsgs {
		if m.WorkspaceID == workspaceID {
			delete(s.inboxMsgs, k)
		}
	}
	return nil
}
