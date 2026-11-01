package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedHandovers struct {
	Items []persistedHandover `json:"items"`
}

type persistedHandover struct {
	ID           string         `json:"id"`
	WorkspaceID  string         `json:"workspaceId"`
	BusinessDate string         `json:"businessDate"`
	FromShift    string         `json:"fromShift"`
	ToShift      string         `json:"toShift"`
	Status       string         `json:"status"`
	Note         string         `json:"note"`
	Tasks        []persistedRef `json:"tasks"`
	Recoveries   []persistedRef `json:"recoveries"`
	Breakdowns   []persistedRef `json:"breakdowns"`
	OpenedBy     string         `json:"openedBy"`
	AcceptedBy   string         `json:"acceptedBy,omitempty"`
	OpenedAt     time.Time      `json:"openedAt"`
	AcceptedAt   time.Time      `json:"acceptedAt,omitempty"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

type persistedRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (s *Store) AttachHandoverPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handoverPersist = path
	return s.loadHandoverLocked()
}

func (s *Store) GetOpenHandover(_ context.Context, workspaceID id.ID) (cafe.Handover, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rec := range s.handovers {
		if rec.WorkspaceID == workspaceID && rec.Open() {
			return rec, nil
		}
	}
	return cafe.Handover{}, errx.NotFound("handover_not_found", "handover not found")
}

func (s *Store) GetHandover(_ context.Context, handoverID id.ID) (cafe.Handover, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.handovers[handoverID]
	if !ok {
		return cafe.Handover{}, errx.NotFound("handover_not_found", "handover not found")
	}
	return rec, nil
}

func (s *Store) SaveHandover(_ context.Context, rec cafe.Handover) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handovers == nil {
		s.handovers = map[id.ID]cafe.Handover{}
	}
	if rec.Open() {
		for _, cur := range s.handovers {
			if cur.WorkspaceID == rec.WorkspaceID && cur.Open() && cur.ID != rec.ID {
				return errx.Conflict("handover_open", "accept the open handover first")
			}
		}
	}
	s.handovers[rec.ID] = rec
	return s.flushHandoverLocked()
}

func (s *Store) ListHandovers(_ context.Context, workspaceID id.ID) ([]cafe.Handover, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.Handover, 0)
	for _, rec := range s.handovers {
		if rec.WorkspaceID == workspaceID {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenedAt.After(out[j].OpenedAt) })
	return out, nil
}

func (s *Store) SnapshotOpenWork(_ context.Context, workspaceID id.ID) ([]cafe.HandoverRef, []cafe.HandoverRef, []cafe.HandoverRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	voices := map[id.ID]cafe.Voice{}
	for _, v := range s.voices {
		if v.WorkspaceID == workspaceID {
			voices[v.ID] = v
		}
	}
	tasks := make([]cafe.HandoverRef, 0)
	for _, t := range s.tasks {
		if t.WorkspaceID != workspaceID || t.Status == work.StatusCompleted || t.Status == work.StatusCancelled {
			continue
		}
		tasks = append(tasks, cafe.HandoverRef{ID: t.ID, Title: t.Title})
	}
	recoveries := make([]cafe.HandoverRef, 0)
	for _, rec := range s.recoveries {
		if rec.WorkspaceID != workspaceID || !rec.Open() {
			continue
		}
		recoveries = append(recoveries, cafe.HandoverRef{ID: rec.ID, Title: cafe.RecoverySnapshotTitle(voices[rec.VoiceID])})
	}
	breakdowns := make([]cafe.HandoverRef, 0)
	for _, r := range s.reports {
		if r.WorkspaceID != workspaceID || r.Kind != opsreport.KindBreakage || !r.Open() {
			continue
		}
		breakdowns = append(breakdowns, cafe.HandoverRef{ID: r.ID, Title: r.Title})
	}
	return tasks, recoveries, breakdowns, nil
}

func (s *Store) loadHandoverLocked() error {
	if s.handoverPersist == "" {
		return nil
	}
	if s.handovers == nil {
		s.handovers = map[id.ID]cafe.Handover{}
	}
	raw, err := os.ReadFile(s.handoverPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedHandovers
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, row := range file.Items {
		hid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		openedBy, _ := id.Parse(row.OpenedBy)
		acceptedBy, _ := id.Parse(row.AcceptedBy)
		s.handovers[hid] = cafe.Handover{
			ID: hid, WorkspaceID: wid, BusinessDate: row.BusinessDate, FromShift: row.FromShift, ToShift: row.ToShift,
			Status: row.Status, Note: row.Note, Tasks: decodeRefs(row.Tasks), Recoveries: decodeRefs(row.Recoveries),
			Breakdowns: decodeRefs(row.Breakdowns), OpenedBy: openedBy, AcceptedBy: acceptedBy,
			OpenedAt: row.OpenedAt, AcceptedAt: row.AcceptedAt, UpdatedAt: row.UpdatedAt,
		}
	}
	return nil
}

func (s *Store) flushHandoverLocked() error {
	if s.handoverPersist == "" {
		return nil
	}
	file := persistedHandovers{Items: make([]persistedHandover, 0, len(s.handovers))}
	for _, rec := range s.handovers {
		file.Items = append(file.Items, persistedHandover{
			ID: rec.ID.String(), WorkspaceID: rec.WorkspaceID.String(), BusinessDate: rec.BusinessDate,
			FromShift: rec.FromShift, ToShift: rec.ToShift, Status: rec.Status, Note: rec.Note,
			Tasks: encodeRefs(rec.Tasks), Recoveries: encodeRefs(rec.Recoveries), Breakdowns: encodeRefs(rec.Breakdowns),
			OpenedBy: rec.OpenedBy.String(), AcceptedBy: omitZero(rec.AcceptedBy),
			OpenedAt: rec.OpenedAt, AcceptedAt: rec.AcceptedAt, UpdatedAt: rec.UpdatedAt,
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.handoverPersist), 0o700); err != nil {
		return err
	}
	tmp := s.handoverPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.handoverPersist)
}

func encodeRefs(in []cafe.HandoverRef) []persistedRef {
	out := make([]persistedRef, 0, len(in))
	for _, r := range in {
		out = append(out, persistedRef{ID: r.ID.String(), Title: r.Title})
	}
	return out
}

func decodeRefs(in []persistedRef) []cafe.HandoverRef {
	out := make([]cafe.HandoverRef, 0, len(in))
	for _, r := range in {
		rid, err := id.Parse(r.ID)
		if err != nil {
			continue
		}
		out = append(out, cafe.HandoverRef{ID: rid, Title: r.Title})
	}
	return out
}

func omitZero(v id.ID) string {
	if v == id.Zero() {
		return ""
	}
	return v.String()
}
