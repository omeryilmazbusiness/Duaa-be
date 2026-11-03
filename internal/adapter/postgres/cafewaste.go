package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const wasteSelect = `SELECT id, workspace_id, recorded_by, business_date::text, station, item,
	coalesce(recipe_id, '00000000-0000-0000-0000-000000000000'), grams, cost, reason, note, recorded_at
	FROM waste_tickets`

func scanWasteTicket(row pgx.Row) (cafe.WasteTicket, error) {
	var t cafe.WasteTicket
	var station, reason string
	err := row.Scan(
		&t.ID, &t.WorkspaceID, &t.RecordedBy, &t.BusinessDate, &station, &t.Item, &t.RecipeID,
		&t.Grams, &t.Cost, &reason, &t.Note, &t.RecordedAt,
	)
	t.Station = cafe.PrepStation(station)
	t.Reason = cafe.WasteReason(reason)
	return t, err
}

func (s *Store) CreateWasteTicket(ctx context.Context, t cafe.WasteTicket) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO waste_tickets (
		id, workspace_id, recorded_by, business_date, station, item, recipe_id, grams, cost, reason, note, recorded_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		t.ID, t.WorkspaceID, t.RecordedBy, t.BusinessDate, string(t.Station), t.Item, nullID(t.RecipeID),
		t.Grams, t.Cost, string(t.Reason), t.Note, t.RecordedAt)
	return err
}

func (s *Store) ListWasteTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.WasteTicket, error) {
	rows, err := s.pool.Query(ctx, wasteSelect+` WHERE workspace_id=$1 AND business_date=$2`, workspaceID, businessDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.WasteTicket, 0)
	for rows.Next() {
		t, err := scanWasteTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type recipeRow struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Station string          `json:"station"`
	Lines   []recipeLineRow `json:"lines"`
}

type recipeLineRow struct {
	Title string `json:"title"`
	Grams int64  `json:"grams"`
}

func (s *Store) GetRecipeBook(ctx context.Context, workspaceID id.ID) (cafe.RecipeBook, error) {
	var rec cafe.RecipeBook
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT workspace_id, items, updated_at, updated_by FROM cafe_recipes WHERE workspace_id=$1`, workspaceID).
		Scan(&rec.WorkspaceID, &raw, &rec.UpdatedAt, &rec.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.RecipeBook{}, cafe.RecipeBookNotFoundError()
	}
	if err != nil {
		return cafe.RecipeBook{}, err
	}
	rec.Items = decodeRecipes(raw)
	return rec, nil
}

func (s *Store) SaveRecipeBook(ctx context.Context, rec cafe.RecipeBook) error {
	raw, err := json.Marshal(encodeRecipes(rec.Items))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO cafe_recipes (workspace_id, items, updated_at, updated_by)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (workspace_id) DO UPDATE SET items=EXCLUDED.items, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by`,
		rec.WorkspaceID, raw, rec.UpdatedAt, rec.UpdatedBy)
	return err
}

func encodeRecipes(in []cafe.Recipe) []recipeRow {
	out := make([]recipeRow, 0, len(in))
	for _, rec := range in {
		lines := make([]recipeLineRow, 0, len(rec.Lines))
		for _, line := range rec.Lines {
			lines = append(lines, recipeLineRow{Title: line.Title, Grams: line.Grams})
		}
		out = append(out, recipeRow{ID: rec.ID.String(), Name: rec.Name, Station: string(rec.Station), Lines: lines})
	}
	return out
}

func decodeRecipes(raw []byte) []cafe.Recipe {
	if len(raw) == 0 {
		return []cafe.Recipe{}
	}
	var rows []recipeRow
	if json.Unmarshal(raw, &rows) != nil {
		return []cafe.Recipe{}
	}
	out := make([]cafe.Recipe, 0, len(rows))
	for _, row := range rows {
		rid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		lines := make([]cafe.RecipeLine, 0, len(row.Lines))
		for _, line := range row.Lines {
			lines = append(lines, cafe.RecipeLine{Title: line.Title, Grams: line.Grams})
		}
		out = append(out, cafe.Recipe{ID: rid, Name: row.Name, Station: cafe.PrepStation(row.Station), Lines: lines})
	}
	return out
}
