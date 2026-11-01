package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedChecks struct {
	Templates []persistedCheckTemplate `json:"templates"`
	Runs      []persistedCheckRun      `json:"runs"`
}

type persistedCheckTemplate struct {
	WorkspaceID string               `json:"workspaceId"`
	Items       []persistedCheckItem `json:"items"`
	UpdatedAt   time.Time            `json:"updatedAt"`
	UpdatedBy   string               `json:"updatedBy"`
}

type persistedCheckItem struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Sort   int    `json:"sort"`
	Active bool   `json:"active"`
}

type persistedCheckRun struct {
	ID           string               `json:"id"`
	WorkspaceID  string               `json:"workspaceId"`
	BusinessDate string               `json:"businessDate"`
	Kind         string               `json:"kind"`
	Marks        []persistedCheckMark `json:"marks"`
	UpdatedAt    time.Time            `json:"updatedAt"`
	UpdatedBy    string               `json:"updatedBy"`
}

type persistedCheckMark struct {
	ItemID string    `json:"itemId"`
	Title  string    `json:"title"`
	DoneBy string    `json:"doneBy"`
	DoneAt time.Time `json:"doneAt"`
}

func checkRunKey(workspaceID id.ID, day string, kind cafe.CheckKind) string {
	return workspaceID.String() + ":" + day + ":" + string(kind)
}

func (s *Store) AttachCheckPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkPersist = path
	return s.loadCheckLocked()
}

func (s *Store) GetChecklistTemplate(_ context.Context, workspaceID id.ID) (cafe.ChecklistTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.checkTemplates[workspaceID]
	if !ok {
		return cafe.ChecklistTemplate{}, errx.NotFound("checklist_not_found", "opening list not found")
	}
	return rec, nil
}

func (s *Store) SaveChecklistTemplate(_ context.Context, rec cafe.ChecklistTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.checkTemplates == nil {
		s.checkTemplates = map[id.ID]cafe.ChecklistTemplate{}
	}
	s.checkTemplates[rec.WorkspaceID] = rec
	return s.flushCheckLocked()
}

func (s *Store) GetChecklistRun(_ context.Context, workspaceID id.ID, businessDate string, kind cafe.CheckKind) (cafe.ChecklistRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.checkRuns[checkRunKey(workspaceID, businessDate, kind)]
	if !ok {
		return cafe.ChecklistRun{}, errx.NotFound("checklist_run_not_found", "opening run not found")
	}
	return rec, nil
}

func (s *Store) SaveChecklistRun(_ context.Context, rec cafe.ChecklistRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.checkRuns == nil {
		s.checkRuns = map[string]cafe.ChecklistRun{}
	}
	s.checkRuns[checkRunKey(rec.WorkspaceID, rec.BusinessDate, rec.Kind)] = rec
	return s.flushCheckLocked()
}

func (s *Store) loadCheckLocked() error {
	if s.checkPersist == "" {
		return nil
	}
	if s.checkTemplates == nil {
		s.checkTemplates = map[id.ID]cafe.ChecklistTemplate{}
	}
	if s.checkRuns == nil {
		s.checkRuns = map[string]cafe.ChecklistRun{}
	}
	raw, err := os.ReadFile(s.checkPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedChecks
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, row := range file.Templates {
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		actor, _ := id.Parse(row.UpdatedBy)
		items := make([]cafe.CheckItem, 0, len(row.Items))
		for _, it := range row.Items {
			iid, err := id.Parse(it.ID)
			if err != nil {
				continue
			}
			items = append(items, cafe.CheckItem{ID: iid, Kind: cafe.CheckKind(it.Kind), Title: it.Title, Sort: it.Sort, Active: it.Active})
		}
		s.checkTemplates[wid] = cafe.ChecklistTemplate{WorkspaceID: wid, Items: items, UpdatedAt: row.UpdatedAt, UpdatedBy: actor}
	}
	for _, row := range file.Runs {
		rid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		actor, _ := id.Parse(row.UpdatedBy)
		marks := make([]cafe.CheckMark, 0, len(row.Marks))
		for _, m := range row.Marks {
			iid, err := id.Parse(m.ItemID)
			if err != nil {
				continue
			}
			did, _ := id.Parse(m.DoneBy)
			marks = append(marks, cafe.CheckMark{ItemID: iid, Title: m.Title, DoneBy: did, DoneAt: m.DoneAt})
		}
		kind := cafe.CheckKind(row.Kind)
		s.checkRuns[checkRunKey(wid, row.BusinessDate, kind)] = cafe.ChecklistRun{
			ID: rid, WorkspaceID: wid, BusinessDate: row.BusinessDate, Kind: kind,
			Marks: marks, UpdatedAt: row.UpdatedAt, UpdatedBy: actor,
		}
	}
	return nil
}

func (s *Store) flushCheckLocked() error {
	if s.checkPersist == "" {
		return nil
	}
	file := persistedChecks{Templates: make([]persistedCheckTemplate, 0, len(s.checkTemplates)), Runs: make([]persistedCheckRun, 0, len(s.checkRuns))}
	for _, rec := range s.checkTemplates {
		items := make([]persistedCheckItem, 0, len(rec.Items))
		for _, it := range rec.Items {
			items = append(items, persistedCheckItem{ID: it.ID.String(), Kind: string(it.Kind), Title: it.Title, Sort: it.Sort, Active: it.Active})
		}
		file.Templates = append(file.Templates, persistedCheckTemplate{
			WorkspaceID: rec.WorkspaceID.String(), Items: items, UpdatedAt: rec.UpdatedAt, UpdatedBy: rec.UpdatedBy.String(),
		})
	}
	for _, rec := range s.checkRuns {
		marks := make([]persistedCheckMark, 0, len(rec.Marks))
		for _, m := range rec.Marks {
			marks = append(marks, persistedCheckMark{ItemID: m.ItemID.String(), Title: m.Title, DoneBy: m.DoneBy.String(), DoneAt: m.DoneAt})
		}
		file.Runs = append(file.Runs, persistedCheckRun{
			ID: rec.ID.String(), WorkspaceID: rec.WorkspaceID.String(), BusinessDate: rec.BusinessDate, Kind: string(rec.Kind),
			Marks: marks, UpdatedAt: rec.UpdatedAt, UpdatedBy: rec.UpdatedBy.String(),
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.checkPersist), 0o700); err != nil {
		return err
	}
	tmp := s.checkPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.checkPersist)
}
