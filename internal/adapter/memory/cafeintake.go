package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedIntakes struct {
	Items []persistedIntake `json:"items"`
}

type persistedIntake struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	BusinessDate string    `json:"businessDate"`
	ShiftName    string    `json:"shiftName"`
	CloseNote    string    `json:"closeNote"`
	WasteNote    string    `json:"wasteNote"`
	IncidentNote string    `json:"incidentNote"`
	AuthorUserID string    `json:"authorUserId"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func intakeKey(workspaceID id.ID, day string) string {
	return workspaceID.String() + ":" + day
}

func (s *Store) AttachIntakePersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.intakePersist = path
	return s.loadIntakeLocked()
}

func (s *Store) GetIntake(_ context.Context, workspaceID id.ID, businessDate string) (cafe.Intake, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.intakes[intakeKey(workspaceID, businessDate)]
	if !ok {
		return cafe.Intake{}, errx.NotFound("intake_not_found", "daily close not found")
	}
	return rec, nil
}

func (s *Store) SaveIntake(_ context.Context, rec cafe.Intake) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.intakes == nil {
		s.intakes = map[string]cafe.Intake{}
	}
	s.intakes[intakeKey(rec.WorkspaceID, rec.BusinessDate)] = rec
	return s.flushIntakeLocked()
}

func (s *Store) ListIntakes(_ context.Context, workspaceID id.ID) ([]cafe.Intake, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.Intake, 0)
	for _, rec := range s.intakes {
		if rec.WorkspaceID == workspaceID {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BusinessDate > out[j].BusinessDate })
	return out, nil
}

func (s *Store) loadIntakeLocked() error {
	if s.intakePersist == "" {
		return nil
	}
	if s.intakes == nil {
		s.intakes = map[string]cafe.Intake{}
	}
	raw, err := os.ReadFile(s.intakePersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedIntakes
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, row := range file.Items {
		iid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		author, _ := id.Parse(row.AuthorUserID)
		s.intakes[intakeKey(wid, row.BusinessDate)] = cafe.Intake{
			ID: iid, WorkspaceID: wid, BusinessDate: row.BusinessDate, ShiftName: row.ShiftName,
			CloseNote: row.CloseNote, WasteNote: row.WasteNote, IncidentNote: row.IncidentNote,
			AuthorUserID: author, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
	}
	return nil
}

func (s *Store) flushIntakeLocked() error {
	if s.intakePersist == "" {
		return nil
	}
	file := persistedIntakes{Items: make([]persistedIntake, 0, len(s.intakes))}
	for _, rec := range s.intakes {
		file.Items = append(file.Items, persistedIntake{
			ID: rec.ID.String(), WorkspaceID: rec.WorkspaceID.String(), BusinessDate: rec.BusinessDate,
			ShiftName: rec.ShiftName, CloseNote: rec.CloseNote, WasteNote: rec.WasteNote, IncidentNote: rec.IncidentNote,
			AuthorUserID: rec.AuthorUserID.String(), CreatedAt: rec.CreatedAt, UpdatedAt: rec.UpdatedAt,
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.intakePersist), 0o700); err != nil {
		return err
	}
	tmp := s.intakePersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.intakePersist)
}
