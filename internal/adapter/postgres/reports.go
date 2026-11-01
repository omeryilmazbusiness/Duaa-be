package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const reportSelect = `SELECT id, workspace_id, reporter_user_id, kind, title, detail, status,
	coalesce(linked_task_id, '00000000-0000-0000-0000-000000000000'),
	coalesce(assignee_user_id, '00000000-0000-0000-0000-000000000000'),
	created_at, updated_at FROM staff_reports`

func scanReport(row pgx.Row) (opsreport.Report, error) {
	var r opsreport.Report
	err := row.Scan(
		&r.ID, &r.WorkspaceID, &r.ReporterUserID, &r.Kind, &r.Title, &r.Detail, &r.Status,
		&r.LinkedTaskID, &r.AssigneeUserID, &r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

func (s *Store) CreateReport(ctx context.Context, r opsreport.Report) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO staff_reports (
		id, workspace_id, reporter_user_id, kind, title, detail, status,
		linked_task_id, assignee_user_id, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		r.ID, r.WorkspaceID, r.ReporterUserID, r.Kind, r.Title, r.Detail, r.Status,
		uuidOrNil(r.LinkedTaskID), uuidOrNil(r.AssigneeUserID), r.CreatedAt, r.UpdatedAt)
	return err
}

func (s *Store) GetReport(ctx context.Context, rid id.ID) (opsreport.Report, error) {
	r, err := scanReport(s.pool.QueryRow(ctx, reportSelect+` WHERE id=$1`, rid))
	if err != nil {
		return opsreport.Report{}, mapErr(err)
	}
	return r, nil
}

func (s *Store) ListReports(ctx context.Context, workspaceID id.ID) ([]opsreport.Report, error) {
	rows, err := s.pool.Query(ctx, reportSelect+` WHERE workspace_id=$1`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]opsreport.Report, 0)
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpdateReport(ctx context.Context, r opsreport.Report) error {
	_, err := s.pool.Exec(ctx, `UPDATE staff_reports SET kind=$2, title=$3, detail=$4, status=$5,
		linked_task_id=$6, assignee_user_id=$7, updated_at=$8 WHERE id=$1 AND workspace_id=$9`,
		r.ID, r.Kind, r.Title, r.Detail, r.Status,
		uuidOrNil(r.LinkedTaskID), uuidOrNil(r.AssigneeUserID), r.UpdatedAt, r.WorkspaceID)
	return err
}
