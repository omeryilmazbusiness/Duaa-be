package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const decisionSelect = `SELECT id, workspace_id, created_by_user_id, source_key, fingerprint, kind, title_key, detail, severity, target_tab, status,
	coalesce(linked_task_id, '00000000-0000-0000-0000-000000000000'), coalesce(assignee_user_id, '00000000-0000-0000-0000-000000000000'),
	snooze_until, created_at, updated_at FROM gm_decisions`

func scanDecision(row pgx.Row) (executive.Decision, error) {
	var d executive.Decision
	var snooze *time.Time
	err := row.Scan(
		&d.ID, &d.WorkspaceID, &d.CreatedByUserID, &d.SourceKey, &d.Fingerprint, &d.Kind, &d.TitleKey, &d.Detail,
		&d.Severity, &d.TargetTab, &d.Status, &d.LinkedTaskID, &d.AssigneeUserID, &snooze, &d.CreatedAt, &d.UpdatedAt,
	)
	d.SnoozeUntil = snooze
	return d, err
}

func (s *Store) CreateDecision(ctx context.Context, d executive.Decision) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO gm_decisions (
		id, workspace_id, created_by_user_id, source_key, fingerprint, kind, title_key, detail, severity, target_tab, status,
		linked_task_id, assignee_user_id, snooze_until, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		d.ID, d.WorkspaceID, d.CreatedByUserID, d.SourceKey, d.Fingerprint, d.Kind, d.TitleKey, d.Detail, d.Severity, d.TargetTab, d.Status,
		uuidOrNil(d.LinkedTaskID), uuidOrNil(d.AssigneeUserID), d.SnoozeUntil, d.CreatedAt, d.UpdatedAt)
	if isUnique(err) {
		return mapDecisionConflict(err)
	}
	return err
}

func (s *Store) GetDecision(ctx context.Context, did id.ID) (executive.Decision, error) {
	d, err := scanDecision(s.pool.QueryRow(ctx, decisionSelect+` WHERE id=$1`, did))
	if err != nil {
		return executive.Decision{}, mapErr(err)
	}
	return d, nil
}

func (s *Store) ListDecisions(ctx context.Context, workspaceID id.ID) ([]executive.Decision, error) {
	rows, err := s.pool.Query(ctx, decisionSelect+` WHERE workspace_id=$1`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]executive.Decision, 0)
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateDecision(ctx context.Context, d executive.Decision) error {
	_, err := s.pool.Exec(ctx, `UPDATE gm_decisions SET fingerprint=$2, kind=$3, title_key=$4, detail=$5, severity=$6, target_tab=$7, status=$8,
		linked_task_id=$9, assignee_user_id=$10, snooze_until=$11, updated_at=$12 WHERE id=$1 AND workspace_id=$13`,
		d.ID, d.Fingerprint, d.Kind, d.TitleKey, d.Detail, d.Severity, d.TargetTab, d.Status,
		uuidOrNil(d.LinkedTaskID), uuidOrNil(d.AssigneeUserID), d.SnoozeUntil, d.UpdatedAt, d.WorkspaceID)
	return err
}

func mapDecisionConflict(err error) error {
	if err == nil {
		return nil
	}
	return errx.Conflict("decision_exists", "an open decision already exists for this source")
}
