package cafe_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestWasteTicketRequiresGramsAndCost(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	wid, uid := id.New(), id.New()
	ticket, err := cafe.NewWasteTicket(wid, uid, "2026-11-02", "Kitchen", " sour dough ", "prep", "over proofed", id.Zero(), 240, 3500, now)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Item != "sour dough" || ticket.Station != cafe.StationKitchen || ticket.Grams != 240 || ticket.Cost != 3500 || ticket.Reason != cafe.WastePrep {
		t.Fatalf("ticket=%+v", ticket)
	}
	if _, err := cafe.NewWasteTicket(wid, uid, "2026-11-02", "kitchen", "bread", "", "", id.Zero(), 0, 3500, now); err == nil {
		t.Fatal("zero grams")
	}
	if _, err := cafe.NewWasteTicket(wid, uid, "2026-11-02", "kitchen", "bread", "", "", id.Zero(), 100, 0, now); err == nil {
		t.Fatal("zero cost must not become a live zero")
	}
	if _, err := cafe.NewWasteTicket(wid, uid, "2026-11-02", "oven", "bread", "", "", id.Zero(), 100, 100, now); err == nil {
		t.Fatal("bad station")
	}
}

func TestSummarizeWasteDoesNotInventAnEmptyLiveDay(t *testing.T) {
	empty := cafe.SummarizeWaste("2026-11-02", nil)
	if empty.Live() || empty.TicketCount != 0 || empty.Cost != 0 || empty.Grams != 0 {
		t.Fatalf("empty day must stay unavailable: %+v", empty)
	}
	wid := id.New()
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	kitchen, _ := cafe.NewWasteTicket(wid, id.New(), "2026-11-02", "kitchen", "milk", "spoilage", "", id.Zero(), 500, 8000, now)
	bar, _ := cafe.NewWasteTicket(wid, id.New(), "2026-11-02", "bar", "syrup", "other", "", id.Zero(), 80, 1500, now)
	old, _ := cafe.NewWasteTicket(wid, id.New(), "2026-11-01", "kitchen", "old", "prep", "", id.Zero(), 10, 10, now)
	day := cafe.SummarizeWaste("2026-11-02", []cafe.WasteTicket{kitchen, bar, old})
	if !day.Live() || day.TicketCount != 2 || day.Grams != 580 || day.Cost != 9500 || day.KitchenGrams != 500 || day.BarGrams != 80 {
		t.Fatalf("day=%+v", day)
	}
}

func TestEmptyRecipeBookIsValidAndDoesNotInventWaste(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	book, err := cafe.ReplaceRecipes(id.New(), id.New(), nil, now)
	if err != nil || len(book.Items) != 0 {
		t.Fatalf("empty book=%+v err=%v", book, err)
	}
	if _, err := cafe.ReplaceRecipes(id.New(), id.New(), []cafe.Recipe{{Name: "Latte", Station: cafe.StationBar}}, now); err == nil {
		t.Fatal("recipe without lines")
	}
	saved, err := cafe.ReplaceRecipes(id.New(), id.New(), []cafe.Recipe{{
		Name: "  Flat White ", Station: "bar",
		Lines: []cafe.RecipeLine{{Title: " milk ", Grams: 180}, {Title: "espresso", Grams: 18}},
	}}, now)
	if err != nil || len(saved.Items) != 1 || saved.Items[0].Name != "Flat White" || saved.Items[0].Lines[0].Title != "milk" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
}
