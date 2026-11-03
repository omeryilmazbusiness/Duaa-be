package cafestock

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	defs  DefinitionStore
	store Repository
	clock clock.Clock
}

func New(defs DefinitionStore, store Repository, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{defs: defs, store: store, clock: clk}
}

type ItemView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Station string `json:"station"`
	Unit    string `json:"unit"`
	OnHand  int64  `json:"onHand"`
	Par     int64  `json:"par"`
	Level   int    `json:"level"`
	Active  bool   `json:"active"`
	Low     bool   `json:"low"`
}

type AdjustmentView struct {
	ID           string `json:"id"`
	ItemID       string `json:"itemId"`
	ItemName     string `json:"itemName"`
	Delta        int64  `json:"delta"`
	OnHandAfter  int64  `json:"onHandAfter"`
	Reason       string `json:"reason"`
	Note         string `json:"note,omitempty"`
	RecordedAt   string `json:"recordedAt"`
	BusinessDate string `json:"businessDate"`
}

type View struct {
	BusinessDate   string           `json:"businessDate"`
	Timezone       string           `json:"timezone"`
	Status         string           `json:"status"`
	ReasonKey      string           `json:"reasonKey,omitempty"`
	ItemCount      int              `json:"itemCount"`
	LowCount       int              `json:"lowCount"`
	Items          []ItemView       `json:"items"`
	Adjustments    []AdjustmentView `json:"adjustments"`
	CanRecord      bool             `json:"canRecord"`
	CanEditCatalog bool             `json:"canEditCatalog"`
}

type ItemDraft struct {
	ID      string
	Name    string
	Station string
	Unit    string
	Par     int64
	Active  bool
}

func (s *Service) Get(ctx context.Context, ac actor.Context, date string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.ViewCafe) {
		return View{}, errx.Forbidden("forbidden", "cannot view stock")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac, def, day)
}

func (s *Service) SaveCatalog(ctx context.Context, ac actor.Context, in []ItemDraft) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !canEditCatalog(ac.Scope) {
		return View{}, errx.Forbidden("forbidden", "cannot edit stock catalog")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	prev, err := s.catalog(ctx, ac.Workspace.ID)
	if err != nil {
		return View{}, err
	}
	items := make([]cafe.StockItem, 0, len(in))
	for _, row := range in {
		it := cafe.StockItem{Name: row.Name, Station: cafe.PrepStation(row.Station), Unit: cafe.StockUnit(row.Unit), ParQty: row.Par, Active: row.Active}
		if row.ID != "" {
			parsed, err := id.Parse(row.ID)
			if err != nil {
				return View{}, errx.Invalid("invalid_stock_item", "stock item is not recorded")
			}
			it.ID = parsed
		}
		items = append(items, it)
	}
	book, err := cafe.ReplaceStockCatalog(ac.Workspace.ID, uid, items, prev, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.SaveStockCatalog(ctx, book); err != nil {
		return View{}, err
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day := finance.BusinessDate(s.clock.Now(), def.Timezone)
	return s.toView(ctx, ac, def, day)
}

func (s *Service) Adjust(ctx context.Context, ac actor.Context, date, itemID, reason, note string, onHand *int64, delta *int64) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.RecordStock) {
		return View{}, errx.Forbidden("forbidden", "cannot record stock")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	iid, err := id.Parse(itemID)
	if err != nil {
		return View{}, cafe.StockItemNotFoundError()
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	book, err := s.catalog(ctx, ac.Workspace.ID)
	if err != nil {
		return View{}, err
	}
	idx := -1
	for i, it := range book.Items {
		if it.ID == iid {
			idx = i
			break
		}
	}
	if idx < 0 || !book.Items[idx].Active {
		return View{}, cafe.StockItemNotFoundError()
	}
	parsedReason, err := cafe.ParseStockAdjustReason(reason)
	if err != nil {
		return View{}, err
	}
	item := book.Items[idx]
	var appliedDelta int64
	switch parsedReason {
	case cafe.StockCount:
		if onHand == nil {
			return View{}, errx.Invalid("invalid_stock_on_hand", "count requires onHand")
		}
		next, d, err := cafe.ApplyStockCount(item, *onHand)
		if err != nil {
			return View{}, err
		}
		item, appliedDelta = next, d
	default:
		if delta == nil {
			return View{}, errx.Invalid("invalid_stock_delta", "adjustment requires delta")
		}
		next, err := cafe.ApplyStockDelta(item, *delta)
		if err != nil {
			return View{}, err
		}
		item, appliedDelta = next, *delta
	}
	book.Items[idx] = item
	book.UpdatedAt = s.clock.Now().UTC()
	book.UpdatedBy = uid
	adj, err := cafe.NewStockAdjustment(ac.Workspace.ID, iid, uid, day, appliedDelta, item.OnHand, string(parsedReason), note, book.UpdatedAt)
	if err != nil {
		return View{}, err
	}
	if err := s.store.SaveStockCatalog(ctx, book); err != nil {
		return View{}, err
	}
	if err := s.store.CreateStockAdjustment(ctx, adj); err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac, def, day)
}

func (s *Service) toView(ctx context.Context, ac actor.Context, def cafe.Definition, day string) (View, error) {
	book, err := s.catalog(ctx, ac.Workspace.ID)
	if err != nil {
		return View{}, err
	}
	adjs, err := s.store.ListStockAdjustments(ctx, ac.Workspace.ID, day)
	if err != nil {
		return View{}, err
	}
	sort.SliceStable(adjs, func(i, j int) bool {
		return adjs[i].RecordedAt.After(adjs[j].RecordedAt)
	})
	sum := cafe.SummarizeStock(day, book)
	view := View{
		BusinessDate:   day,
		Timezone:       def.Timezone,
		Status:         "unavailable",
		ReasonKey:      "stock.noCatalog",
		Items:          make([]ItemView, 0, len(book.Items)),
		Adjustments:    make([]AdjustmentView, 0, len(adjs)),
		CanRecord:      rbac.Allows(ac.Scope, rbac.RecordStock),
		CanEditCatalog: canEditCatalog(ac.Scope),
	}
	names := map[string]string{}
	for _, it := range book.Items {
		names[it.ID.String()] = it.Name
		if !it.Active {
			continue
		}
		level := cafe.StockLevel(it.OnHand, it.ParQty)
		low := it.ParQty > 0 && it.OnHand < it.ParQty
		view.Items = append(view.Items, ItemView{
			ID: it.ID.String(), Name: it.Name, Station: string(it.Station), Unit: string(it.Unit),
			OnHand: it.OnHand, Par: it.ParQty, Level: level, Active: true, Low: low,
		})
	}
	for _, adj := range adjs {
		view.Adjustments = append(view.Adjustments, AdjustmentView{
			ID: adj.ID.String(), ItemID: adj.ItemID.String(), ItemName: names[adj.ItemID.String()],
			Delta: adj.Delta, OnHandAfter: adj.OnHandAfter, Reason: string(adj.Reason), Note: adj.Note,
			RecordedAt: adj.RecordedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), BusinessDate: adj.BusinessDate,
		})
	}
	if sum.Live() {
		view.Status = "live"
		view.ReasonKey = ""
		view.ItemCount = sum.ItemCount
		view.LowCount = sum.LowCount
	}
	return view, nil
}

func (s *Service) catalog(ctx context.Context, workspaceID id.ID) (cafe.StockCatalog, error) {
	rec, err := s.store.GetStockCatalog(ctx, workspaceID)
	if err == nil {
		return rec, nil
	}
	if errors.Is(err, errx.ErrNotFound) {
		return cafe.StockCatalog{WorkspaceID: workspaceID, Items: []cafe.StockItem{}}, nil
	}
	return cafe.StockCatalog{}, err
}

func (s *Service) definition(ctx context.Context, ac actor.Context) (cafe.Definition, error) {
	def, err := s.defs.GetCafeDefinition(ctx, ac.Workspace.ID)
	if err == nil {
		return def, nil
	}
	if errors.Is(err, errx.ErrNotFound) {
		return cafe.Seed(ac.Workspace.ID, ac.Workspace.Name, ac.Workspace.Timezone), nil
	}
	return cafe.Definition{}, err
}

func (s *Service) day(def cafe.Definition, raw string) (string, error) {
	if raw == "" {
		return finance.BusinessDate(s.clock.Now(), def.Timezone), nil
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil || len(raw) != 10 {
		return "", errx.Invalid("invalid_business_date", "business date must be YYYY-MM-DD")
	}
	return raw, nil
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "stock is cafe-only")
	}
	return nil
}

func canEditCatalog(scope staffscope.Kind) bool {
	return scope == staffscope.GeneralManager || scope == staffscope.DepartmentManager
}
