package cafe_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestIntakeRejectsEmptyAndInventedShifts(t *testing.T) {
	now := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	_, err := cafe.UpsertIntake(nil, id.New(), id.New(), "2026-11-01", "", "", "", "", nil, now)
	if err == nil {
		t.Fatal("empty intake must fail")
	}
	_, err = cafe.UpsertIntake(nil, id.New(), id.New(), "2026-11-01", "Night", "", "bread leftover", "", nil, now)
	if err == nil {
		t.Fatal("must not invent a shift when none are recorded")
	}
	got, err := cafe.UpsertIntake(nil, id.New(), id.New(), "2026-11-01", "", "quiet close", "", "", nil, now)
	if err != nil || got.WasteNote != "" || got.CloseNote != "quiet close" {
		t.Fatalf("close only=%+v err=%v", got, err)
	}
}

func TestIntakeKeepsTheSameFileOnTheSameDay(t *testing.T) {
	now := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	wid, author := id.New(), id.New()
	first, err := cafe.UpsertIntake(nil, wid, author, "2026-11-01", "Morning", "ok", "", "", []string{"Morning"}, now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := cafe.UpsertIntake(&first, wid, author, "2026-11-01", "Morning", "ok", "milk spill", "", []string{"Morning"}, now.Add(time.Hour))
	if err != nil || next.ID != first.ID || next.WasteNote != "milk spill" || next.CreatedAt != first.CreatedAt {
		t.Fatalf("upsert=%+v err=%v", next, err)
	}
}

func TestParseBusinessDate(t *testing.T) {
	if _, err := cafe.ParseBusinessDate("01-11-2026"); err == nil {
		t.Fatal("must be YYYY-MM-DD")
	}
	d, err := cafe.ParseBusinessDate(" 2026-11-01 ")
	if err != nil || d != "2026-11-01" {
		t.Fatalf("date=%s err=%v", d, err)
	}
}
