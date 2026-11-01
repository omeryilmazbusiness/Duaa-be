package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) GetChecklistTemplate(ctx context.Context, workspaceID id.ID) (cafe.ChecklistTemplate, error) {
	var rec cafe.ChecklistTemplate
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT workspace_id, items, updated_at, updated_by FROM cafe_checklist_templates WHERE workspace_id=$1`, workspaceID).
		Scan(&rec.WorkspaceID, &raw, &rec.UpdatedAt, &rec.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.ChecklistTemplate{}, errx.NotFound("checklist_not_found", "opening list not found")
	}
	if err != nil {
		return cafe.ChecklistTemplate{}, err
	}
	rec.Items = decodeCheckItems(raw)
	return rec, nil
}

func (s *Store) SaveChecklistTemplate(ctx context.Context, rec cafe.ChecklistTemplate) error {
	raw, err := json.Marshal(encodeCheckItems(rec.Items))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO cafe_checklist_templates (workspace_id, items, updated_at, updated_by)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (workspace_id) DO UPDATE SET items=EXCLUDED.items, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by`,
		rec.WorkspaceID, raw, rec.UpdatedAt, rec.UpdatedBy)
	return err
}

func (s *Store) GetChecklistRun(ctx context.Context, workspaceID id.ID, businessDate string, kind cafe.CheckKind) (cafe.ChecklistRun, error) {
	var rec cafe.ChecklistRun
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT id, workspace_id, business_date::text, kind, marks, updated_at, updated_by
		FROM cafe_checklist_runs WHERE workspace_id=$1 AND business_date=$2 AND kind=$3`, workspaceID, businessDate, kind).
		Scan(&rec.ID, &rec.WorkspaceID, &rec.BusinessDate, &rec.Kind, &raw, &rec.UpdatedAt, &rec.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.ChecklistRun{}, errx.NotFound("checklist_run_not_found", "opening run not found")
	}
	if err != nil {
		return cafe.ChecklistRun{}, err
	}
	rec.Marks = decodeCheckMarks(raw)
	return rec, nil
}

func (s *Store) SaveChecklistRun(ctx context.Context, rec cafe.ChecklistRun) error {
	raw, err := json.Marshal(encodeCheckMarks(rec.Marks))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO cafe_checklist_runs (id, workspace_id, business_date, kind, marks, updated_at, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (workspace_id, business_date, kind) DO UPDATE SET
			marks=EXCLUDED.marks, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by`,
		rec.ID, rec.WorkspaceID, rec.BusinessDate, rec.Kind, raw, rec.UpdatedAt, rec.UpdatedBy)
	return err
}

type checkItemRow struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Sort   int    `json:"sort"`
	Active bool   `json:"active"`
}

type checkMarkRow struct {
	ItemID string `json:"itemId"`
	Title  string `json:"title"`
	DoneBy string `json:"doneBy"`
	DoneAt string `json:"doneAt"`
}

func encodeCheckItems(in []cafe.CheckItem) []checkItemRow {
	out := make([]checkItemRow, 0, len(in))
	for _, it := range in {
		out = append(out, checkItemRow{ID: it.ID.String(), Kind: string(it.Kind), Title: it.Title, Sort: it.Sort, Active: it.Active})
	}
	return out
}

func decodeCheckItems(raw []byte) []cafe.CheckItem {
	if len(raw) == 0 {
		return []cafe.CheckItem{}
	}
	var rows []checkItemRow
	if json.Unmarshal(raw, &rows) != nil {
		return []cafe.CheckItem{}
	}
	out := make([]cafe.CheckItem, 0, len(rows))
	for _, row := range rows {
		iid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		out = append(out, cafe.CheckItem{ID: iid, Kind: cafe.CheckKind(row.Kind), Title: row.Title, Sort: row.Sort, Active: row.Active})
	}
	return out
}

func encodeCheckMarks(in []cafe.CheckMark) []checkMarkRow {
	out := make([]checkMarkRow, 0, len(in))
	for _, m := range in {
		out = append(out, checkMarkRow{ItemID: m.ItemID.String(), Title: m.Title, DoneBy: m.DoneBy.String(), DoneAt: m.DoneAt.UTC().Format("2006-01-02T15:04:05Z07:00")})
	}
	return out
}

func decodeCheckMarks(raw []byte) []cafe.CheckMark {
	if len(raw) == 0 {
		return []cafe.CheckMark{}
	}
	var rows []checkMarkRow
	if json.Unmarshal(raw, &rows) != nil {
		return []cafe.CheckMark{}
	}
	out := make([]cafe.CheckMark, 0, len(rows))
	for _, row := range rows {
		iid, err := id.Parse(row.ItemID)
		if err != nil {
			continue
		}
		did, _ := id.Parse(row.DoneBy)
		mark := cafe.CheckMark{ItemID: iid, Title: row.Title, DoneBy: did}
		if t, err := time.Parse(time.RFC3339, row.DoneAt); err == nil {
			mark.DoneAt = t
		}
		out = append(out, mark)
	}
	return out
}
