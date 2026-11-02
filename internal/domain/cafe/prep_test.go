package cafe_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestPrepTicketOpensWaitingAndRejectsBadInput(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	wid, uid := id.New(), id.New()
	ticket, err := cafe.NewPrepTicket(wid, uid, "2026-11-02", "Kitchen", " T-12 ", " 8 ", now)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Station != cafe.StationKitchen || ticket.TicketRef != "t-12" || ticket.TableLabel != "8" || ticket.Status() != cafe.PrepWaiting {
		t.Fatalf("ticket=%+v", ticket)
	}
	if ticket.WaitMinutes(now.Add(4*time.Minute)) != 4 {
		t.Fatalf("wait=%d", ticket.WaitMinutes(now.Add(4*time.Minute)))
	}
	if _, err := cafe.NewPrepTicket(wid, uid, "2026-11-02", "oven", "t-1", "", now); err == nil {
		t.Fatal("bad station")
	}
	if _, err := cafe.NewPrepTicket(wid, uid, "2026-11-02", "bar", "", "", now); err == nil {
		t.Fatal("empty ref")
	}
}

func TestPrepReadyThenServeAndRejectOutOfOrder(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	ticket, err := cafe.NewPrepTicket(id.New(), id.New(), "2026-11-02", "bar", "b-1", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ticket.MarkServed(id.New(), now.Add(time.Minute)); err == nil {
		t.Fatal("serve before ready")
	}
	cook := id.New()
	if err := ticket.MarkReady(cook, now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if ticket.Status() != cafe.PrepReady || ticket.WaitMinutes(now.Add(20*time.Minute)) != 6 {
		t.Fatalf("ready=%+v wait=%d", ticket, ticket.WaitMinutes(now.Add(20*time.Minute)))
	}
	if err := ticket.MarkReady(cook, now.Add(7*time.Minute)); err == nil {
		t.Fatal("double ready")
	}
	floor := id.New()
	if err := ticket.MarkServed(floor, now.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if ticket.Status() != cafe.PrepServed || ticket.Open() || ticket.OpenMinutes(now) != 9 {
		t.Fatalf("served=%+v", ticket)
	}
}

func TestSummarizePrepDoesNotInventAnEmptyLiveDay(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 12, 0, 0, time.UTC)
	empty := cafe.SummarizePrep("2026-11-02", nil, now)
	if empty.Live() || empty.TicketCount != 0 || empty.LongestOpenMinutes != 0 {
		t.Fatalf("empty day must stay unavailable: %+v", empty)
	}
	wid := id.New()
	waiting, _ := cafe.NewPrepTicket(wid, id.New(), "2026-11-02", "kitchen", "k1", "12", now.Add(-8*time.Minute))
	ready, _ := cafe.NewPrepTicket(wid, id.New(), "2026-11-02", "bar", "b1", "", now.Add(-5*time.Minute))
	_ = ready.MarkReady(id.New(), now.Add(-2*time.Minute))
	old, _ := cafe.NewPrepTicket(wid, id.New(), "2026-11-01", "kitchen", "old", "", now)
	day := cafe.SummarizePrep("2026-11-02", []cafe.PrepTicket{waiting, ready, old}, now)
	if !day.Live() || day.WaitingCount != 1 || day.ReadyCount != 1 || day.ServedCount != 0 || day.OpenCount() != 2 {
		t.Fatalf("day=%+v", day)
	}
	if day.LongestOpenMinutes != 8 {
		t.Fatalf("longest=%d", day.LongestOpenMinutes)
	}
}

func TestPrepClockCannotRunBackwards(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	ticket, err := cafe.NewPrepTicket(id.New(), id.New(), "2026-11-02", "kitchen", "k2", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ticket.MarkReady(id.New(), now.Add(-time.Minute)); !errors.Is(err, errx.ErrInvalid) {
		t.Fatalf("ready before received: %v", err)
	}
}
