package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type stockItemRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Station string `json:"station"`
	Unit    string `json:"unit"`
	ParQty  int64  `json:"parQty"`
	OnHand  int64  `json:"onHand"`
	Active  bool   `json:"active"`
}

func (s *Store) GetStockCatalog(ctx context.Context, workspaceID id.ID) (cafe.StockCatalog, error) {
	var rec cafe.StockCatalog
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT workspace_id, items, updated_at, updated_by FROM cafe_stock_catalog WHERE workspace_id=$1`, workspaceID).
		Scan(&rec.WorkspaceID, &raw, &rec.UpdatedAt, &rec.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.StockCatalog{}, cafe.StockCatalogNotFoundError()
	}
	if err != nil {
		return cafe.StockCatalog{}, err
	}
	rec.Items = decodeStockItems(raw)
	return rec, nil
}

func (s *Store) SaveStockCatalog(ctx context.Context, rec cafe.StockCatalog) error {
	raw, err := json.Marshal(encodeStockItems(rec.Items))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO cafe_stock_catalog (workspace_id, items, updated_at, updated_by)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (workspace_id) DO UPDATE SET items=EXCLUDED.items, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by`,
		rec.WorkspaceID, raw, rec.UpdatedAt, rec.UpdatedBy)
	return err
}

func (s *Store) CreateStockAdjustment(ctx context.Context, adj cafe.StockAdjustment) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO stock_adjustments (
		id, workspace_id, item_id, recorded_by, business_date, delta, on_hand_after, reason, note, recorded_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		adj.ID, adj.WorkspaceID, adj.ItemID, adj.RecordedBy, adj.BusinessDate, adj.Delta, adj.OnHandAfter,
		string(adj.Reason), adj.Note, adj.RecordedAt)
	return err
}

func (s *Store) ListStockAdjustments(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.StockAdjustment, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, workspace_id, item_id, recorded_by, business_date::text, delta, on_hand_after, reason, note, recorded_at
		FROM stock_adjustments WHERE workspace_id=$1 AND business_date=$2`, workspaceID, businessDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.StockAdjustment, 0)
	for rows.Next() {
		var adj cafe.StockAdjustment
		var reason string
		if err := rows.Scan(&adj.ID, &adj.WorkspaceID, &adj.ItemID, &adj.RecordedBy, &adj.BusinessDate, &adj.Delta, &adj.OnHandAfter, &reason, &adj.Note, &adj.RecordedAt); err != nil {
			return nil, err
		}
		adj.Reason = cafe.StockAdjustReason(reason)
		out = append(out, adj)
	}
	return out, rows.Err()
}

func encodeStockItems(in []cafe.StockItem) []stockItemRow {
	out := make([]stockItemRow, 0, len(in))
	for _, it := range in {
		out = append(out, stockItemRow{
			ID: it.ID.String(), Name: it.Name, Station: string(it.Station), Unit: string(it.Unit),
			ParQty: it.ParQty, OnHand: it.OnHand, Active: it.Active,
		})
	}
	return out
}

func decodeStockItems(raw []byte) []cafe.StockItem {
	if len(raw) == 0 {
		return nil
	}
	var rows []stockItemRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil
	}
	out := make([]cafe.StockItem, 0, len(rows))
	for _, row := range rows {
		iid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		out = append(out, cafe.StockItem{
			ID: iid, Name: row.Name, Station: cafe.PrepStation(row.Station), Unit: cafe.StockUnit(row.Unit),
			ParQty: row.ParQty, OnHand: row.OnHand, Active: row.Active,
		})
	}
	return out
}
