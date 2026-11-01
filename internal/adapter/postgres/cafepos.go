package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const posTicketSelect = `SELECT id, workspace_id, recorded_by, external_ref, business_date::text, kind, lines, gross, discount, net, tender, recorded_at FROM pos_tickets`

func scanPOSTicket(row pgx.Row) (cafe.Ticket, error) {
	var t cafe.Ticket
	var kind, tender string
	err := row.Scan(
		&t.ID, &t.WorkspaceID, &t.RecordedBy, &t.ExternalRef, &t.BusinessDate, &kind, &t.Lines,
		&t.Gross, &t.Discount, &t.Net, &tender, &t.RecordedAt,
	)
	t.Kind = cafe.TicketKind(kind)
	t.Tender = cafe.TicketTender(tender)
	return t, err
}

func (s *Store) CreatePOSTicket(ctx context.Context, t cafe.Ticket) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO pos_tickets (
		id, workspace_id, recorded_by, external_ref, business_date, kind, lines, gross, discount, net, tender, recorded_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		t.ID, t.WorkspaceID, t.RecordedBy, t.ExternalRef, t.BusinessDate, string(t.Kind), t.Lines, t.Gross, t.Discount, t.Net, string(t.Tender), t.RecordedAt)
	if isUnique(err) {
		return cafe.DuplicateTicketConflict()
	}
	return err
}

func (s *Store) ListPOSTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.Ticket, error) {
	rows, err := s.pool.Query(ctx, posTicketSelect+` WHERE workspace_id=$1 AND business_date=$2`, workspaceID, businessDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.Ticket, 0)
	for rows.Next() {
		t, err := scanPOSTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetPOSClose(ctx context.Context, workspaceID id.ID, businessDate string) (cafe.POSClose, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, workspace_id, closed_by, business_date::text, ticket_count, sale_count, return_count, gross, discount, returns, net, basket, closed_at
		FROM pos_closes WHERE workspace_id=$1 AND business_date=$2`, workspaceID, businessDate)
	var c cafe.POSClose
	err := row.Scan(
		&c.ID, &c.WorkspaceID, &c.ClosedBy, &c.BusinessDate, &c.TicketCount, &c.SaleCount, &c.ReturnCount,
		&c.Gross, &c.Discount, &c.Returns, &c.Net, &c.Basket, &c.ClosedAt,
	)
	if err != nil {
		return cafe.POSClose{}, mapErr(err)
	}
	return c, nil
}

func (s *Store) CreatePOSClose(ctx context.Context, c cafe.POSClose) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO pos_closes (
		id, workspace_id, closed_by, business_date, ticket_count, sale_count, return_count, gross, discount, returns, net, basket, closed_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		c.ID, c.WorkspaceID, c.ClosedBy, c.BusinessDate, c.TicketCount, c.SaleCount, c.ReturnCount, c.Gross, c.Discount, c.Returns, c.Net, c.Basket, c.ClosedAt)
	if isUnique(err) {
		return cafe.POSDayClosedConflict()
	}
	return err
}
