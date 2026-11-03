package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedWaste struct {
	Tickets []persistedWasteTicket `json:"tickets"`
	Books   []persistedRecipeBook  `json:"books"`
}

type persistedWasteTicket struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	RecordedBy   string    `json:"recordedBy"`
	BusinessDate string    `json:"businessDate"`
	Station      string    `json:"station"`
	Item         string    `json:"item"`
	RecipeID     string    `json:"recipeId,omitempty"`
	Grams        int64     `json:"grams"`
	Cost         int64     `json:"cost"`
	Reason       string    `json:"reason"`
	Note         string    `json:"note,omitempty"`
	RecordedAt   time.Time `json:"recordedAt"`
}

type persistedRecipeBook struct {
	WorkspaceID string            `json:"workspaceId"`
	Items       []persistedRecipe `json:"items"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	UpdatedBy   string            `json:"updatedBy"`
}

type persistedRecipe struct {
	ID      string                `json:"id"`
	Name    string                `json:"name"`
	Station string                `json:"station"`
	Lines   []persistedRecipeLine `json:"lines"`
}

type persistedRecipeLine struct {
	Title string `json:"title"`
	Grams int64  `json:"grams"`
}

func (s *Store) AttachWastePersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wastePersist = path
	return s.loadWasteLocked()
}

func (s *Store) CreateWasteTicket(_ context.Context, t cafe.WasteTicket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wasteTickets == nil {
		s.wasteTickets = map[id.ID]cafe.WasteTicket{}
	}
	s.wasteTickets[t.ID] = t
	return s.flushWasteLocked()
}

func (s *Store) ListWasteTickets(_ context.Context, workspaceID id.ID, businessDate string) ([]cafe.WasteTicket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.WasteTicket, 0)
	for _, t := range s.wasteTickets {
		if t.WorkspaceID == workspaceID && t.BusinessDate == businessDate {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *Store) GetRecipeBook(_ context.Context, workspaceID id.ID) (cafe.RecipeBook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.recipeBooks[workspaceID]
	if !ok {
		return cafe.RecipeBook{}, cafe.RecipeBookNotFoundError()
	}
	return rec, nil
}

func (s *Store) SaveRecipeBook(_ context.Context, rec cafe.RecipeBook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recipeBooks == nil {
		s.recipeBooks = map[id.ID]cafe.RecipeBook{}
	}
	s.recipeBooks[rec.WorkspaceID] = rec
	return s.flushWasteLocked()
}

func (s *Store) loadWasteLocked() error {
	if s.wastePersist == "" {
		return nil
	}
	if s.wasteTickets == nil {
		s.wasteTickets = map[id.ID]cafe.WasteTicket{}
	}
	if s.recipeBooks == nil {
		s.recipeBooks = map[id.ID]cafe.RecipeBook{}
	}
	raw, err := os.ReadFile(s.wastePersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedWaste
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, row := range file.Tickets {
		tid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		recorder, _ := id.Parse(row.RecordedBy)
		recipeID, _ := id.Parse(row.RecipeID)
		s.wasteTickets[tid] = cafe.WasteTicket{
			ID: tid, WorkspaceID: wid, RecordedBy: recorder, BusinessDate: row.BusinessDate,
			Station: cafe.PrepStation(row.Station), Item: row.Item, RecipeID: recipeID,
			Grams: row.Grams, Cost: row.Cost, Reason: cafe.WasteReason(row.Reason),
			Note: row.Note, RecordedAt: row.RecordedAt,
		}
	}
	for _, row := range file.Books {
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		updatedBy, _ := id.Parse(row.UpdatedBy)
		items := make([]cafe.Recipe, 0, len(row.Items))
		for _, rec := range row.Items {
			rid, err := id.Parse(rec.ID)
			if err != nil {
				continue
			}
			lines := make([]cafe.RecipeLine, 0, len(rec.Lines))
			for _, line := range rec.Lines {
				lines = append(lines, cafe.RecipeLine{Title: line.Title, Grams: line.Grams})
			}
			items = append(items, cafe.Recipe{ID: rid, Name: rec.Name, Station: cafe.PrepStation(rec.Station), Lines: lines})
		}
		s.recipeBooks[wid] = cafe.RecipeBook{WorkspaceID: wid, Items: items, UpdatedAt: row.UpdatedAt, UpdatedBy: updatedBy}
	}
	return nil
}

func (s *Store) flushWasteLocked() error {
	if s.wastePersist == "" {
		return nil
	}
	file := persistedWaste{
		Tickets: make([]persistedWasteTicket, 0, len(s.wasteTickets)),
		Books:   make([]persistedRecipeBook, 0, len(s.recipeBooks)),
	}
	for _, t := range s.wasteTickets {
		file.Tickets = append(file.Tickets, persistedWasteTicket{
			ID: t.ID.String(), WorkspaceID: t.WorkspaceID.String(), RecordedBy: t.RecordedBy.String(),
			BusinessDate: t.BusinessDate, Station: string(t.Station), Item: t.Item, RecipeID: idString(t.RecipeID),
			Grams: t.Grams, Cost: t.Cost, Reason: string(t.Reason), Note: t.Note, RecordedAt: t.RecordedAt,
		})
	}
	for _, b := range s.recipeBooks {
		items := make([]persistedRecipe, 0, len(b.Items))
		for _, rec := range b.Items {
			lines := make([]persistedRecipeLine, 0, len(rec.Lines))
			for _, line := range rec.Lines {
				lines = append(lines, persistedRecipeLine{Title: line.Title, Grams: line.Grams})
			}
			items = append(items, persistedRecipe{ID: rec.ID.String(), Name: rec.Name, Station: string(rec.Station), Lines: lines})
		}
		file.Books = append(file.Books, persistedRecipeBook{
			WorkspaceID: b.WorkspaceID.String(), Items: items, UpdatedAt: b.UpdatedAt, UpdatedBy: b.UpdatedBy.String(),
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.wastePersist), 0o700); err != nil {
		return err
	}
	tmp := s.wastePersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.wastePersist)
}
