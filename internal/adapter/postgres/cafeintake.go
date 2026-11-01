package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const intakeSelect = `SELECT id, workspace_id, business_date::text, shift_name, close_note, waste_note, incident_note, author_user_id, created_at, updated_at FROM cafe_intakes`

func scanIntake(row pgx.Row) (cafe.Intake, error) {
	var rec cafe.Intake
	err := row.Scan(
		&rec.ID, &rec.WorkspaceID, &rec.BusinessDate, &rec.ShiftName, &rec.CloseNote, &rec.WasteNote,
		&rec.IncidentNote, &rec.AuthorUserID, &rec.CreatedAt, &rec.UpdatedAt,
	)
	return rec, err
}

func (s *Store) GetIntake(ctx context.Context, workspaceID id.ID, businessDate string) (cafe.Intake, error) {
	rec, err := scanIntake(s.pool.QueryRow(ctx, intakeSelect+` WHERE workspace_id=$1 AND business_date=$2`, workspaceID, businessDate))
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.Intake{}, errx.NotFound("intake_not_found", "daily close not found")
	}
	if err != nil {
		return cafe.Intake{}, err
	}
	return rec, nil
}

func (s *Store) SaveIntake(ctx context.Context, rec cafe.Intake) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO cafe_intakes (
			id, workspace_id, business_date, shift_name, close_note, waste_note, incident_note, author_user_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (workspace_id, business_date) DO UPDATE SET
			shift_name=EXCLUDED.shift_name, close_note=EXCLUDED.close_note, waste_note=EXCLUDED.waste_note,
			incident_note=EXCLUDED.incident_note, author_user_id=EXCLUDED.author_user_id, updated_at=EXCLUDED.updated_at`,
		rec.ID, rec.WorkspaceID, rec.BusinessDate, rec.ShiftName, rec.CloseNote, rec.WasteNote, rec.IncidentNote,
		rec.AuthorUserID, rec.CreatedAt, rec.UpdatedAt)
	return err
}

func (s *Store) ListIntakes(ctx context.Context, workspaceID id.ID) ([]cafe.Intake, error) {
	rows, err := s.pool.Query(ctx, intakeSelect+` WHERE workspace_id=$1 ORDER BY business_date DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.Intake, 0)
	for rows.Next() {
		rec, err := scanIntake(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
