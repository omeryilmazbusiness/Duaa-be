package cafe_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestHandoverAllowsAnEmptySnapshotAndRejectsInventedShifts(t *testing.T) {
	now := time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)
	h, err := cafe.OpenHandover(id.New(), id.New(), "2026-11-02", "", "", "", nil, nil, nil, nil, now)
	if err != nil || !h.Open() || len(h.Tasks) != 0 {
		t.Fatalf("empty handoff is valid: %+v err=%v", h, err)
	}
	if _, err := cafe.OpenHandover(id.New(), id.New(), "2026-11-02", "Night", "", "", nil, nil, nil, nil, now); err == nil {
		t.Fatal("must not invent a shift")
	}
	item := cafe.HandoverRef{ID: id.New(), Title: "  restock milk  "}
	got, err := cafe.OpenHandover(id.New(), id.New(), "2026-11-02", "Morning", "Evening", "", []string{"Morning", "Evening"}, []cafe.HandoverRef{item, item, {Title: "ghost"}}, nil, nil, now)
	if err != nil || len(got.Tasks) != 1 || got.Tasks[0].Title != "restock milk" {
		t.Fatalf("dedupe=%+v err=%v", got, err)
	}
}

func TestHandoverAcceptOnce(t *testing.T) {
	now := time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)
	h, err := cafe.OpenHandover(id.New(), id.New(), "2026-11-02", "", "", "keys on bar", nil, nil, nil, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	incoming := id.New()
	if err := h.Accept(incoming, now.Add(time.Hour)); err != nil || h.Status != cafe.HandoverAccepted || h.AcceptedBy != incoming {
		t.Fatalf("accept=%+v err=%v", h, err)
	}
	if err := h.Accept(id.New(), now.Add(2*time.Hour)); err == nil {
		t.Fatal("accepted file stays closed")
	}
}
