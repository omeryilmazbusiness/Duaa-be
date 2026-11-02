package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const prepSelect = `SELECT id, workspace_id, opened_by,
	coalesce(ready_by, '00000000-0000-0000-0000-000000000000'),
	coalesce(served_by, '00000000-0000-0000-0000-000000000000'),
	business_date::text, station, ticket_ref, table_label, received_at,
	coalesce(ready_at, '0001-01-01'), coalesce(served_at, '0001-01-01')
	FROM prep_tickets`

func scanPrepTicket(row pgx.Row) (cafe.PrepTicket, error) {
	var t cafe.PrepTicket
	var station string
	err := row.Scan(
		&t.ID, &t.WorkspaceID, &t.OpenedBy, &t.ReadyBy, &t.ServedBy, &t.BusinessDate, &station,
		&t.TicketRef, &t.TableLabel, &t.ReceivedAt, &t.ReadyAt, &t.ServedAt,
	)
	t.Station = cafe.PrepStation(station)
	return t, err
}

func (s *Store) CreatePrepTicket(ctx context.Context, t cafe.PrepTicket) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO prep_tickets (
		id, workspace_id, opened_by, ready_by, served_by, business_date, station, ticket_ref, table_label, received_at, ready_at, served_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		t.ID, t.WorkspaceID, t.OpenedBy, nullID(t.ReadyBy), nullID(t.ServedBy), t.BusinessDate, string(t.Station),
		t.TicketRef, t.TableLabel, t.ReceivedAt, nullTime(t.ReadyAt), nullTime(t.ServedAt))
	if isUnique(err) {
		return cafe.DuplicatePrepConflict()
	}
	return err
}

func (s *Store) GetPrepTicket(ctx context.Context, tid id.ID) (cafe.PrepTicket, error) {
	t, err := scanPrepTicket(s.pool.QueryRow(ctx, prepSelect+` WHERE id=$1`, tid))
	if err != nil {
		if mapped := mapErr(err); mapped != err {
			return cafe.PrepTicket{}, cafe.PrepNotFoundError()
		}
		return cafe.PrepTicket{}, err
	}
	return t, nil
}

func (s *Store) ListPrepTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.PrepTicket, error) {
	rows, err := s.pool.Query(ctx, prepSelect+` WHERE workspace_id=$1 AND business_date=$2`, workspaceID, businessDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.PrepTicket, 0)
	for rows.Next() {
		t, err := scanPrepTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) SavePrepTicket(ctx context.Context, t cafe.PrepTicket) error {
	tag, err := s.pool.Exec(ctx, `UPDATE prep_tickets SET
		ready_by=$2, served_by=$3, ready_at=$4, served_at=$5
		WHERE id=$1`,
		t.ID, nullID(t.ReadyBy), nullID(t.ServedBy), nullTime(t.ReadyAt), nullTime(t.ServedAt))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return cafe.PrepNotFoundError()
	}
	return nil
}

func nullID(v id.ID) any {
	if v == id.Zero() {
		return nil
	}
	return v
}
