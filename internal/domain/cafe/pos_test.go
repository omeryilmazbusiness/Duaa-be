package cafe_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestTicketComputesNetAndRejectsBadDiscount(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	wid, uid := id.New(), id.New()
	sale, err := cafe.NewTicket(wid, uid, "2026-11-02", " T-100 ", "sale", "cash", 2, 10000, 1000, now)
	if err != nil {
		t.Fatal(err)
	}
	if sale.Net != 9000 || sale.ExternalRef != "t-100" || sale.Kind != cafe.TicketSale {
		t.Fatalf("sale=%+v", sale)
	}
	if _, err := cafe.NewTicket(wid, uid, "2026-11-02", "t-101", "sale", "cash", 1, 1000, 2000, now); err == nil {
		t.Fatal("discount > gross")
	}
	if _, err := cafe.NewTicket(wid, uid, "2026-11-02", "", "sale", "cash", 1, 1000, 0, now); err == nil {
		t.Fatal("empty ref")
	}
}

func TestSummarizeDoesNotInventAnEmptyLiveDay(t *testing.T) {
	empty := cafe.SummarizeTickets("2026-11-02", nil)
	if empty.Live() || empty.Net != 0 || empty.TicketCount != 0 {
		t.Fatalf("empty day must stay unavailable: %+v", empty)
	}
	wid := id.New()
	now := time.Now().UTC()
	sale, _ := cafe.NewTicket(wid, id.New(), "2026-11-02", "s1", "sale", "card", 3, 15000, 0, now)
	ret, _ := cafe.NewTicket(wid, id.New(), "2026-11-02", "r1", "return", "card", 1, 2000, 0, now)
	other, _ := cafe.NewTicket(wid, id.New(), "2026-11-01", "old", "sale", "cash", 1, 999, 0, now)
	day := cafe.SummarizeTickets("2026-11-02", []cafe.Ticket{sale, ret, other})
	if !day.Live() || day.SaleCount != 1 || day.ReturnCount != 1 || day.Net != 13000 || day.Returns != 2000 || day.Basket != 15000 {
		t.Fatalf("day=%+v", day)
	}
	if day.Card != 13000 || day.Cash != 0 {
		t.Fatalf("tender=%+v", day)
	}
}

func TestEmptyPOSDayCannotClose(t *testing.T) {
	_, err := cafe.ClosePOSDay(id.New(), id.New(), "2026-11-02", nil, time.Now().UTC())
	if err == nil || !errors.Is(err, errx.ErrInvalid) {
		t.Fatalf("empty close: %v", err)
	}
}

func TestCloseFreezesRecordedTotals(t *testing.T) {
	wid, uid := id.New(), id.New()
	now := time.Now().UTC()
	sale, err := cafe.NewTicket(wid, uid, "2026-11-02", "s1", "sale", "cash", 2, 8000, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := cafe.ClosePOSDay(wid, uid, "2026-11-02", []cafe.Ticket{sale}, now)
	if err != nil {
		t.Fatal(err)
	}
	got := closed.DaySales()
	if !got.Closed || got.Net != 8000 || got.Basket != 8000 || !got.Live() {
		t.Fatalf("close=%+v", got)
	}
}
