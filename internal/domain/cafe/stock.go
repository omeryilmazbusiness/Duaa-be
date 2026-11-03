package cafe

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type StockUnit string
type StockAdjustReason string

const (
	UnitGrams  StockUnit = "g"
	UnitMillil StockUnit = "ml"
	UnitEach   StockUnit = "ea"
)

const (
	StockCount   StockAdjustReason = "count"
	StockReceive StockAdjustReason = "receive"
	StockUse     StockAdjustReason = "use"
	StockSpoil   StockAdjustReason = "spoil"
	StockOther   StockAdjustReason = "other"
)

const (
	MaxStockItems    = 48
	MaxStockName     = 80
	MinStockName     = 2
	MaxStockNote     = 200
	MaxStockQty      = 1_000_000_000
	MaxStockAdjust   = 1_000_000_000
)

// StockItem is one recorded SKU with on-hand truth. Levels are derived from onHand/par — never invented.
type StockItem struct {
	ID      id.ID
	Name    string
	Station PrepStation
	Unit    StockUnit
	ParQty  int64
	OnHand  int64
	Active  bool
}

// StockCatalog is the workspace SKU book. Empty is valid (view stays unavailable).
type StockCatalog struct {
	WorkspaceID id.ID
	Items       []StockItem
	UpdatedAt   time.Time
	UpdatedBy   id.ID
}

// StockAdjustment is one recorded movement or absolute count.
type StockAdjustment struct {
	ID           id.ID
	WorkspaceID  id.ID
	ItemID       id.ID
	RecordedBy   id.ID
	BusinessDate string
	Delta        int64
	OnHandAfter  int64
	Reason       StockAdjustReason
	Note         string
	RecordedAt   time.Time
}

// DayStock is derived only from the recorded catalog.
type DayStock struct {
	BusinessDate string
	ItemCount    int
	LowCount     int
	ActiveCount  int
}

func ParseStockUnit(raw string) (StockUnit, error) {
	switch StockUnit(strings.ToLower(strings.TrimSpace(raw))) {
	case UnitGrams, UnitMillil, UnitEach:
		return StockUnit(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", errx.Invalid("invalid_stock_unit", "unit must be g, ml or ea")
	}
}

func ParseStockAdjustReason(raw string) (StockAdjustReason, error) {
	switch StockAdjustReason(strings.ToLower(strings.TrimSpace(raw))) {
	case "":
		return StockOther, nil
	case StockCount, StockReceive, StockUse, StockSpoil, StockOther:
		return StockAdjustReason(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", errx.Invalid("invalid_stock_reason", "reason must be count, receive, use, spoil or other")
	}
}

func NormalizeStockName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// StockLevel returns on-hand as 0–100 of par. Par unset → 100 if any on-hand, else 0.
func StockLevel(onHand, par int64) int {
	if onHand < 0 {
		onHand = 0
	}
	if par > 0 {
		pct := int((onHand * 100) / par)
		if pct > 100 {
			return 100
		}
		if pct < 0 {
			return 0
		}
		return pct
	}
	if onHand > 0 {
		return 100
	}
	return 0
}

func (c *StockCatalog) Normalize() {
	if c == nil {
		return
	}
	items := make([]StockItem, 0, len(c.Items))
	seenID := map[id.ID]struct{}{}
	seenName := map[string]struct{}{}
	for _, it := range c.Items {
		it.Name = clip(strings.TrimSpace(it.Name), MaxStockName)
		station, err := ParsePrepStation(string(it.Station))
		if err != nil {
			continue
		}
		it.Station = station
		unit, err := ParseStockUnit(string(it.Unit))
		if err != nil {
			continue
		}
		it.Unit = unit
		if it.ID == id.Zero() {
			it.ID = id.New()
		}
		if _, ok := seenID[it.ID]; ok {
			continue
		}
		key := NormalizeStockName(it.Name)
		if key == "" {
			continue
		}
		if _, ok := seenName[key]; ok {
			continue
		}
		if it.ParQty < 0 || it.ParQty > MaxStockQty {
			it.ParQty = 0
		}
		if it.OnHand < 0 {
			it.OnHand = 0
		}
		if it.OnHand > MaxStockQty {
			it.OnHand = MaxStockQty
		}
		seenID[it.ID] = struct{}{}
		seenName[key] = struct{}{}
		items = append(items, it)
		if len(items) == MaxStockItems {
			break
		}
	}
	c.Items = items
}

func (c StockCatalog) Validate() error {
	if c.WorkspaceID == id.Zero() || c.UpdatedBy == id.Zero() {
		return errx.Invalid("invalid_stock_catalog", "workspace and author are required")
	}
	if len(c.Items) > MaxStockItems {
		return errx.Invalid("stock_catalog_too_long", "too many stock items")
	}
	for _, it := range c.Items {
		if utf8.RuneCountInString(it.Name) < MinStockName {
			return errx.Invalid("invalid_stock_name", "write the item name")
		}
		if _, err := ParsePrepStation(string(it.Station)); err != nil {
			return err
		}
		if _, err := ParseStockUnit(string(it.Unit)); err != nil {
			return err
		}
		if it.ParQty < 0 || it.ParQty > MaxStockQty {
			return errx.Invalid("invalid_stock_par", "par quantity is invalid")
		}
		if it.OnHand < 0 || it.OnHand > MaxStockQty {
			return errx.Invalid("invalid_stock_on_hand", "on-hand quantity is invalid")
		}
	}
	return nil
}

func (c StockCatalog) Item(itemID id.ID) (StockItem, bool) {
	for _, it := range c.Items {
		if it.ID == itemID {
			return it, true
		}
	}
	return StockItem{}, false
}

func ReplaceStockCatalog(workspaceID, author id.ID, items []StockItem, previous StockCatalog, now time.Time) (StockCatalog, error) {
	// Catalog edits never overwrite recorded on-hand — only Adjust does.
	prevOnHand := map[id.ID]int64{}
	for _, it := range previous.Items {
		prevOnHand[it.ID] = it.OnHand
	}
	outItems := make([]StockItem, 0, len(items))
	for _, it := range items {
		if it.ID != id.Zero() {
			if oh, ok := prevOnHand[it.ID]; ok {
				it.OnHand = oh
			}
		}
		outItems = append(outItems, it)
	}
	book := StockCatalog{
		WorkspaceID: workspaceID,
		Items:       outItems,
		UpdatedAt:   now,
		UpdatedBy:   author,
	}
	book.Normalize()
	if err := book.Validate(); err != nil {
		return StockCatalog{}, err
	}
	return book, nil
}

func ApplyStockCount(item StockItem, onHand int64) (StockItem, int64, error) {
	if onHand < 0 || onHand > MaxStockQty {
		return StockItem{}, 0, errx.Invalid("invalid_stock_on_hand", "on-hand quantity is invalid")
	}
	delta := onHand - item.OnHand
	item.OnHand = onHand
	return item, delta, nil
}

func ApplyStockDelta(item StockItem, delta int64) (StockItem, error) {
	if delta == 0 || delta > MaxStockAdjust || delta < -MaxStockAdjust {
		return StockItem{}, errx.Invalid("invalid_stock_delta", "adjustment delta must be recorded")
	}
	next := item.OnHand + delta
	if next < 0 || next > MaxStockQty {
		return StockItem{}, errx.Invalid("invalid_stock_on_hand", "on-hand would leave valid range")
	}
	item.OnHand = next
	return item, nil
}

func NewStockAdjustment(workspaceID, itemID, recorder id.ID, date string, delta, onHandAfter int64, reason, note string, now time.Time) (StockAdjustment, error) {
	if workspaceID == id.Zero() || itemID == id.Zero() || recorder == id.Zero() {
		return StockAdjustment{}, errx.Invalid("invalid_stock_refs", "workspace, item and recorder are required")
	}
	if !validBusinessDate(date) {
		return StockAdjustment{}, errx.Invalid("invalid_business_date", "business date must be YYYY-MM-DD")
	}
	parsed, err := ParseStockAdjustReason(reason)
	if err != nil {
		return StockAdjustment{}, err
	}
	if now.IsZero() {
		return StockAdjustment{}, errx.Invalid("invalid_stock_clock", "recorded time is required")
	}
	return StockAdjustment{
		ID:           id.New(),
		WorkspaceID:  workspaceID,
		ItemID:       itemID,
		RecordedBy:   recorder,
		BusinessDate: date,
		Delta:        delta,
		OnHandAfter:  onHandAfter,
		Reason:       parsed,
		Note:         clip(strings.TrimSpace(note), MaxStockNote),
		RecordedAt:   now,
	}, nil
}

func SummarizeStock(date string, catalog StockCatalog) DayStock {
	out := DayStock{BusinessDate: date}
	for _, it := range catalog.Items {
		if !it.Active {
			continue
		}
		out.ActiveCount++
		out.ItemCount++
		if it.ParQty > 0 && it.OnHand < it.ParQty {
			out.LowCount++
		}
	}
	return out
}

func (s DayStock) Live() bool { return s.ActiveCount > 0 }

func StockCatalogNotFoundError() error {
	return errx.NotFound("stock_catalog_not_found", "stock catalog not found")
}

func StockItemNotFoundError() error {
	return errx.NotFound("stock_item_not_found", "stock item not found")
}
