package cafe_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestEmptyTemplateIsValid(t *testing.T) {
	rec, err := cafe.ReplaceTemplate(id.New(), id.New(), nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rec.Validate() != nil || len(rec.ActiveItems(cafe.CheckOpen)) != 0 {
		t.Fatal("empty list is valid — do not invent opening lines")
	}
}

func TestMaintenanceKindStaysClosed(t *testing.T) {
	if _, err := cafe.ParseCheckKind("maintenance"); err == nil {
		t.Fatal("maintenance photos come later")
	}
	rec, err := cafe.ReplaceTemplate(id.New(), id.New(), []cafe.CheckItem{
		{Kind: "maintenance", Title: "photo the fridge", Active: true},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Items) != 0 {
		t.Fatal("maintenance lines must drop, not persist as a fake open item")
	}
}

func TestUnknownKindRejected(t *testing.T) {
	if _, err := cafe.ParseCheckKind("prep"); err == nil {
		t.Fatal("invented kind")
	}
}

func TestToggleDoesNotInventALine(t *testing.T) {
	wid, actor := id.New(), id.New()
	_, err := cafe.ToggleRun(nil, wid, actor, "2026-11-02", cafe.CheckItem{Kind: cafe.CheckOpen, Title: "lights", Active: true}, time.Now())
	if err == nil {
		t.Fatal("unrecorded line must not tick")
	}
}

func TestToggleRoundTripAndUnmark(t *testing.T) {
	wid, actor := id.New(), id.New()
	item := cafe.CheckItem{ID: id.New(), Kind: cafe.CheckOpen, Title: "chairs up", Active: true}
	run, err := cafe.ToggleRun(nil, wid, actor, "2026-11-02", item, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !run.Marked(item.ID) || run.Remaining([]cafe.CheckItem{item}) != 0 {
		t.Fatal("tick must record the line")
	}
	if run.Marks[0].Title != "chairs up" {
		t.Fatal("title is frozen at tick")
	}
	undone, err := cafe.ToggleRun(&run, wid, actor, "2026-11-02", item, time.Now().Add(time.Minute))
	if err != nil || undone.Marked(item.ID) {
		t.Fatal("second tap unmarks")
	}
}

func TestInactiveLineCannotTick(t *testing.T) {
	item := cafe.CheckItem{ID: id.New(), Kind: cafe.CheckClose, Title: "safe", Active: false}
	if _, err := cafe.ToggleRun(nil, id.New(), id.New(), "2026-11-02", item, time.Now()); err == nil {
		t.Fatal("inactive line is not on today's list")
	}
}

func TestTemplateDropsBlankAndCapsTitle(t *testing.T) {
	rec, err := cafe.ReplaceTemplate(id.New(), id.New(), []cafe.CheckItem{
		{Kind: cafe.CheckOpen, Title: "  ", Active: true},
		{Kind: cafe.CheckClose, Title: "lock " + string(make([]byte, 200)), Active: true},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Items) != 1 || len(rec.Items[0].Title) > cafe.MaxCheckTitle {
		t.Fatalf("items=%d title=%d", len(rec.Items), len(rec.Items[0].Title))
	}
}

func TestTooManyLinesRejected(t *testing.T) {
	items := make([]cafe.CheckItem, cafe.MaxCheckItems+1)
	for i := range items {
		items[i] = cafe.CheckItem{Kind: cafe.CheckOpen, Title: "line", Active: true}
	}
	if _, err := cafe.ReplaceTemplate(id.New(), id.New(), items, time.Now()); err == nil {
		t.Fatal("cap the list")
	}
}
