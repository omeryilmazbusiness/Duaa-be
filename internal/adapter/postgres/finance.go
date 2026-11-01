package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) GetCloseByDate(ctx context.Context, workspaceID id.ID, businessDate string) (finance.Close, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, workspace_id, business_date::text, counted_amount, expected_amount, variance, note,
		closed_by_user_id, created_at, updated_at FROM cash_closes WHERE workspace_id=$1 AND business_date=$2`,
		workspaceID, businessDate)
	c, err := scanClose(row)
	if err != nil {
		return finance.Close{}, mapErr(err)
	}
	return c, nil
}

func (s *Store) CreateClose(ctx context.Context, c finance.Close) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO cash_closes (
		id, workspace_id, business_date, counted_amount, expected_amount, variance, note, closed_by_user_id, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		c.ID, c.WorkspaceID, c.BusinessDate, c.CountedAmount, c.ExpectedAmount, c.Variance, c.Note, c.ClosedByUserID, c.CreatedAt, c.UpdatedAt)
	return err
}

func (s *Store) UpdateClose(ctx context.Context, c finance.Close) error {
	_, err := s.pool.Exec(ctx, `UPDATE cash_closes SET counted_amount=$2, expected_amount=$3, variance=$4, note=$5, updated_at=$6
		WHERE id=$1 AND workspace_id=$7`,
		c.ID, c.CountedAmount, c.ExpectedAmount, c.Variance, c.Note, c.UpdatedAt, c.WorkspaceID)
	return err
}

func scanClose(row pgx.Row) (finance.Close, error) {
	var c finance.Close
	err := row.Scan(
		&c.ID, &c.WorkspaceID, &c.BusinessDate, &c.CountedAmount, &c.ExpectedAmount, &c.Variance, &c.Note,
		&c.ClosedByUserID, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

const approvalSelect = `SELECT id, workspace_id, created_by_user_id, title, supplier, amount, currency, evidence, fingerprint, version, status, created_at, updated_at FROM spend_approvals`

func scanApproval(row pgx.Row) (finance.Approval, error) {
	var a finance.Approval
	err := row.Scan(
		&a.ID, &a.WorkspaceID, &a.CreatedByUserID, &a.Title, &a.Supplier, &a.Amount, &a.Currency, &a.Evidence,
		&a.Fingerprint, &a.Version, &a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	return a, err
}

func (s *Store) CreateApproval(ctx context.Context, a finance.Approval) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO spend_approvals (
		id, workspace_id, created_by_user_id, title, supplier, amount, currency, evidence, fingerprint, version, status, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		a.ID, a.WorkspaceID, a.CreatedByUserID, a.Title, a.Supplier, a.Amount, a.Currency, a.Evidence, a.Fingerprint, a.Version, a.Status, a.CreatedAt, a.UpdatedAt)
	if isUnique(err) {
		return finance.DuplicateConflict()
	}
	return err
}

func (s *Store) GetApproval(ctx context.Context, aid id.ID) (finance.Approval, error) {
	a, err := scanApproval(s.pool.QueryRow(ctx, approvalSelect+` WHERE id=$1`, aid))
	if err != nil {
		return finance.Approval{}, mapErr(err)
	}
	return a, nil
}

func (s *Store) ListApprovals(ctx context.Context, workspaceID id.ID) ([]finance.Approval, error) {
	rows, err := s.pool.Query(ctx, approvalSelect+` WHERE workspace_id=$1`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]finance.Approval, 0)
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) UpdateApproval(ctx context.Context, a finance.Approval) error {
	_, err := s.pool.Exec(ctx, `UPDATE spend_approvals SET title=$2, supplier=$3, amount=$4, currency=$5, evidence=$6, fingerprint=$7, version=$8, status=$9, updated_at=$10
		WHERE id=$1 AND workspace_id=$11`,
		a.ID, a.Title, a.Supplier, a.Amount, a.Currency, a.Evidence, a.Fingerprint, a.Version, a.Status, a.UpdatedAt, a.WorkspaceID)
	if isUnique(err) {
		return finance.DuplicateConflict()
	}
	return err
}
