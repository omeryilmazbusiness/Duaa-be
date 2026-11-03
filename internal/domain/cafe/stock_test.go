package cafe_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestStockLevel(t *testing.T) {
	t.Parallel()
	if cafe.StockLevel(50, 100) != 50 {
		t.Fatalf("expected 50")
	}
	if cafe.StockLevel(200, 100) != 100 {
		t.Fatalf("capped at 100")
	}
	if cafe.StockLevel(0, 0) != 0 || cafe.StockLevel(3, 0) != 100 {
		t.Fatalf("par unset uses presence")
	}
}

func TestReplaceStockCatalogPreservesOnHand(t *testing.T) {
	t.Parallel()
	wid, author := id.New(), id.New()
	itemID := id.New()
	prev := cafe.StockCatalog{WorkspaceID: wid, Items: []cafe.StockItem{{
		ID: itemID, Name: "Milk", Station: cafe.StationBar, Unit: cafe.UnitMillil, ParQty: 1000, OnHand: 420, Active: true,
	}}}
	got, err := cafe.ReplaceStockCatalog(wid, author, []cafe.StockItem{{
		ID: itemID, Name: "Milk", Station: cafe.StationBar, Unit: cafe.UnitMillil, ParQty: 1200, OnHand: 0, Active: true,
	}}, prev, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].OnHand != 420 || got.Items[0].ParQty != 1200 {
		t.Fatalf("got %+v", got.Items)
	}
}

func TestApplyStockDeltaAndCount(t *testing.T) {
	t.Parallel()
	item := cafe.StockItem{OnHand: 10, ParQty: 20, Active: true}
	next, delta, err := cafe.ApplyStockCount(item, 7)
	if err != nil || next.OnHand != 7 || delta != -3 {
		t.Fatalf("count %v %d %v", next, delta, err)
	}
	next, err = cafe.ApplyStockDelta(item, 5)
	if err != nil || next.OnHand != 15 {
		t.Fatalf("delta %v %v", next, err)
	}
	if _, err := cafe.ApplyStockDelta(item, -20); err == nil {
		t.Fatal("expected underflow")
	}
}

func TestSummarizeStockLive(t *testing.T) {
	t.Parallel()
	sum := cafe.SummarizeStock("2026-11-03", cafe.StockCatalog{Items: []cafe.StockItem{
		{Name: "Beans", Station: cafe.StationBar, Unit: cafe.UnitGrams, ParQty: 100, OnHand: 40, Active: true},
		{Name: "Old", Station: cafe.StationBar, Unit: cafe.UnitEach, Active: false},
	}})
	if !sum.Live() || sum.ItemCount != 1 || sum.LowCount != 1 {
		t.Fatalf("%+v", sum)
	}
	if cafe.SummarizeStock("2026-11-03", cafe.StockCatalog{}).Live() {
		t.Fatal("empty must stay unavailable")
	}
}
