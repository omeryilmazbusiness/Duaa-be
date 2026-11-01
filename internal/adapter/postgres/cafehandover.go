package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const handoverSelect = `SELECT id, workspace_id, business_date::text, from_shift, to_shift, status, note, tasks, recoveries, breakdowns, opened_by, coalesce(accepted_by, '00000000-0000-0000-0000-000000000000'), opened_at, accepted_at, updated_at FROM cafe_handovers`

func scanHandover(row pgx.Row) (cafe.Handover, error) {
	var rec cafe.Handover
	var tasksRaw, recsRaw, breaksRaw []byte
	var acceptedAt *time.Time
	err := row.Scan(
		&rec.ID, &rec.WorkspaceID, &rec.BusinessDate, &rec.FromShift, &rec.ToShift, &rec.Status, &rec.Note,
		&tasksRaw, &recsRaw, &breaksRaw, &rec.OpenedBy, &rec.AcceptedBy, &rec.OpenedAt, &acceptedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return cafe.Handover{}, err
	}
	rec.Tasks = decodeHandoverRefs(tasksRaw)
	rec.Recoveries = decodeHandoverRefs(recsRaw)
	rec.Breakdowns = decodeHandoverRefs(breaksRaw)
	if acceptedAt != nil {
		rec.AcceptedAt = *acceptedAt
	}
	return rec, nil
}

func (s *Store) GetOpenHandover(ctx context.Context, workspaceID id.ID) (cafe.Handover, error) {
	rec, err := scanHandover(s.pool.QueryRow(ctx, handoverSelect+` WHERE workspace_id=$1 AND status=$2`, workspaceID, cafe.HandoverOpen))
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.Handover{}, errx.NotFound("handover_not_found", "handover not found")
	}
	if err != nil {
		return cafe.Handover{}, err
	}
	return rec, nil
}

func (s *Store) GetHandover(ctx context.Context, handoverID id.ID) (cafe.Handover, error) {
	rec, err := scanHandover(s.pool.QueryRow(ctx, handoverSelect+` WHERE id=$1`, handoverID))
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.Handover{}, errx.NotFound("handover_not_found", "handover not found")
	}
	if err != nil {
		return cafe.Handover{}, err
	}
	return rec, nil
}

func (s *Store) SaveHandover(ctx context.Context, rec cafe.Handover) error {
	tasks, err := json.Marshal(rec.Tasks)
	if err != nil {
		return err
	}
	recoveries, err := json.Marshal(rec.Recoveries)
	if err != nil {
		return err
	}
	breakdowns, err := json.Marshal(rec.Breakdowns)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO cafe_handovers (
			id, workspace_id, business_date, from_shift, to_shift, status, note, tasks, recoveries, breakdowns,
			opened_by, accepted_by, opened_at, accepted_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			status=EXCLUDED.status, note=EXCLUDED.note, accepted_by=EXCLUDED.accepted_by,
			accepted_at=EXCLUDED.accepted_at, updated_at=EXCLUDED.updated_at`,
		rec.ID, rec.WorkspaceID, rec.BusinessDate, rec.FromShift, rec.ToShift, rec.Status, rec.Note,
		tasks, recoveries, breakdowns, rec.OpenedBy, uuidOrNil(rec.AcceptedBy), rec.OpenedAt, nullTime(rec.AcceptedAt), rec.UpdatedAt)
	if isUnique(err) {
		return errx.Conflict("handover_open", "accept the open handover first")
	}
	return err
}

func (s *Store) ListHandovers(ctx context.Context, workspaceID id.ID) ([]cafe.Handover, error) {
	rows, err := s.pool.Query(ctx, handoverSelect+` WHERE workspace_id=$1 ORDER BY opened_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.Handover, 0)
	for rows.Next() {
		rec, err := scanHandover(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) SnapshotOpenWork(ctx context.Context, workspaceID id.ID) ([]cafe.HandoverRef, []cafe.HandoverRef, []cafe.HandoverRef, error) {
	tasks, err := queryHandoverRefs(ctx, s, `SELECT id, title FROM staff_tasks WHERE workspace_id=$1 AND status <> $2 AND status <> $3`,
		workspaceID, work.StatusCompleted, work.StatusCancelled)
	if err != nil {
		return nil, nil, nil, err
	}
	recoveries, err := queryHandoverRefs(ctx, s, `SELECT r.id, CASE WHEN length(btrim(v.comment)) > 0 THEN left(btrim(v.comment), 80) ELSE v.kind END
		FROM cafe_recoveries r JOIN cafe_voices v ON v.id = r.voice_id
		WHERE r.workspace_id=$1 AND r.status IN ($2,$3)`,
		workspaceID, cafe.RecoveryOpen, cafe.RecoveryAssigned)
	if err != nil {
		return nil, nil, nil, err
	}
	breakdowns, err := queryHandoverRefs(ctx, s, `SELECT id, title FROM staff_reports WHERE workspace_id=$1 AND kind=$2 AND status IN ($3,$4)`,
		workspaceID, opsreport.KindBreakage, opsreport.StatusOpen, opsreport.StatusAssigned)
	if err != nil {
		return nil, nil, nil, err
	}
	return tasks, recoveries, breakdowns, nil
}

func queryHandoverRefs(ctx context.Context, s *Store, sql string, args ...any) ([]cafe.HandoverRef, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.HandoverRef, 0)
	for rows.Next() {
		var ref cafe.HandoverRef
		if err := rows.Scan(&ref.ID, &ref.Title); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func decodeHandoverRefs(raw []byte) []cafe.HandoverRef {
	if len(raw) == 0 {
		return []cafe.HandoverRef{}
	}
	var out []cafe.HandoverRef
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []cafe.HandoverRef{}
	}
	return out
}
