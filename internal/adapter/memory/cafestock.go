package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedStock struct {
	Catalogs    []persistedStockCatalog    `json:"catalogs"`
	Adjustments []persistedStockAdjustment `json:"adjustments"`
}

type persistedStockCatalog struct {
	WorkspaceID string              `json:"workspaceId"`
	Items       []persistedStockItem `json:"items"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	UpdatedBy   string              `json:"updatedBy"`
}

type persistedStockItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Station string `json:"station"`
	Unit    string `json:"unit"`
	ParQty  int64  `json:"parQty"`
	OnHand  int64  `json:"onHand"`
	Active  bool   `json:"active"`
}

type persistedStockAdjustment struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	ItemID       string    `json:"itemId"`
	RecordedBy   string    `json:"recordedBy"`
	BusinessDate string    `json:"businessDate"`
	Delta        int64     `json:"delta"`
	OnHandAfter  int64     `json:"onHandAfter"`
	Reason       string    `json:"reason"`
	Note         string    `json:"note,omitempty"`
	RecordedAt   time.Time `json:"recordedAt"`
}

func (s *Store) AttachStockPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stockPersist = path
	return s.loadStockLocked()
}

func (s *Store) GetStockCatalog(_ context.Context, workspaceID id.ID) (cafe.StockCatalog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.stockCatalogs[workspaceID]
	if !ok {
		return cafe.StockCatalog{}, cafe.StockCatalogNotFoundError()
	}
	return rec, nil
}

func (s *Store) SaveStockCatalog(_ context.Context, rec cafe.StockCatalog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stockCatalogs == nil {
		s.stockCatalogs = map[id.ID]cafe.StockCatalog{}
	}
	s.stockCatalogs[rec.WorkspaceID] = rec
	return s.flushStockLocked()
}

func (s *Store) CreateStockAdjustment(_ context.Context, adj cafe.StockAdjustment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stockAdjusts == nil {
		s.stockAdjusts = map[id.ID]cafe.StockAdjustment{}
	}
	s.stockAdjusts[adj.ID] = adj
	return s.flushStockLocked()
}

func (s *Store) ListStockAdjustments(_ context.Context, workspaceID id.ID, businessDate string) ([]cafe.StockAdjustment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.StockAdjustment, 0)
	for _, adj := range s.stockAdjusts {
		if adj.WorkspaceID == workspaceID && adj.BusinessDate == businessDate {
			out = append(out, adj)
		}
	}
	return out, nil
}

func (s *Store) flushStockLocked() error {
	if s.stockPersist == "" {
		return nil
	}
	payload := persistedStock{Catalogs: make([]persistedStockCatalog, 0, len(s.stockCatalogs)), Adjustments: make([]persistedStockAdjustment, 0, len(s.stockAdjusts))}
	for _, rec := range s.stockCatalogs {
		items := make([]persistedStockItem, 0, len(rec.Items))
		for _, it := range rec.Items {
			items = append(items, persistedStockItem{
				ID: it.ID.String(), Name: it.Name, Station: string(it.Station), Unit: string(it.Unit),
				ParQty: it.ParQty, OnHand: it.OnHand, Active: it.Active,
			})
		}
		payload.Catalogs = append(payload.Catalogs, persistedStockCatalog{
			WorkspaceID: rec.WorkspaceID.String(), Items: items, UpdatedAt: rec.UpdatedAt, UpdatedBy: rec.UpdatedBy.String(),
		})
	}
	for _, adj := range s.stockAdjusts {
		payload.Adjustments = append(payload.Adjustments, persistedStockAdjustment{
			ID: adj.ID.String(), WorkspaceID: adj.WorkspaceID.String(), ItemID: adj.ItemID.String(), RecordedBy: adj.RecordedBy.String(),
			BusinessDate: adj.BusinessDate, Delta: adj.Delta, OnHandAfter: adj.OnHandAfter, Reason: string(adj.Reason),
			Note: adj.Note, RecordedAt: adj.RecordedAt,
		})
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.stockPersist), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.stockPersist, raw, 0o600)
}

func (s *Store) loadStockLocked() error {
	if s.stockPersist == "" {
		return nil
	}
	raw, err := os.ReadFile(s.stockPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload persistedStock
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	s.stockCatalogs = map[id.ID]cafe.StockCatalog{}
	s.stockAdjusts = map[id.ID]cafe.StockAdjustment{}
	for _, row := range payload.Catalogs {
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		author, err := id.Parse(row.UpdatedBy)
		if err != nil {
			continue
		}
		items := make([]cafe.StockItem, 0, len(row.Items))
		for _, it := range row.Items {
			iid, err := id.Parse(it.ID)
			if err != nil {
				continue
			}
			items = append(items, cafe.StockItem{
				ID: iid, Name: it.Name, Station: cafe.PrepStation(it.Station), Unit: cafe.StockUnit(it.Unit),
				ParQty: it.ParQty, OnHand: it.OnHand, Active: it.Active,
			})
		}
		s.stockCatalogs[wid] = cafe.StockCatalog{WorkspaceID: wid, Items: items, UpdatedAt: row.UpdatedAt, UpdatedBy: author}
	}
	for _, row := range payload.Adjustments {
		aid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		iid, err := id.Parse(row.ItemID)
		if err != nil {
			continue
		}
		rid, err := id.Parse(row.RecordedBy)
		if err != nil {
			continue
		}
		s.stockAdjusts[aid] = cafe.StockAdjustment{
			ID: aid, WorkspaceID: wid, ItemID: iid, RecordedBy: rid, BusinessDate: row.BusinessDate,
			Delta: row.Delta, OnHandAfter: row.OnHandAfter, Reason: cafe.StockAdjustReason(row.Reason),
			Note: row.Note, RecordedAt: row.RecordedAt,
		}
	}
	return nil
}
