package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedMeetings struct {
	Meetings []assistant.MeetingRecord `json:"meetings"`
}

func (s *Store) AttachMeetingPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meetingPersist = path
	return s.loadMeetingLocked()
}

func (s *Store) SaveMeeting(_ context.Context, rec assistant.MeetingRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.meetings == nil {
		s.meetings = map[id.ID]assistant.MeetingRecord{}
	}
	s.meetings[rec.ID] = rec
	return s.flushMeetingLocked()
}

func (s *Store) ListMeetings(_ context.Context, workspaceID id.ID, limit int) ([]assistant.MeetingRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]assistant.MeetingRecord, 0)
	for _, m := range s.meetings {
		if m.WorkspaceID == workspaceID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) flushMeetingLocked() error {
	if s.meetingPersist == "" {
		return nil
	}
	payload := persistedMeetings{Meetings: make([]assistant.MeetingRecord, 0, len(s.meetings))}
	for _, m := range s.meetings {
		payload.Meetings = append(payload.Meetings, m)
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.meetingPersist), 0o700); err != nil {
		return err
	}
	tmp := s.meetingPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.meetingPersist)
}

func (s *Store) loadMeetingLocked() error {
	if s.meetingPersist == "" {
		return nil
	}
	raw, err := os.ReadFile(s.meetingPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload persistedMeetings
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	s.meetings = map[id.ID]assistant.MeetingRecord{}
	for _, m := range payload.Meetings {
		if m.ID != id.Zero() {
			s.meetings[m.ID] = m
		}
	}
	return nil
}
