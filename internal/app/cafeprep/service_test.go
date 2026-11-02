package cafeprep_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafeprep"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type fake struct {
	def     cafe.Definition
	tickets []cafe.PrepTicket
}

func (f *fake) GetCafeDefinition(_ context.Context, _ id.ID) (cafe.Definition, error) {
	if f.def.WorkspaceID == id.Zero() {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "missing")
	}
	return f.def, nil
}
func (f *fake) CreatePrepTicket(_ context.Context, t cafe.PrepTicket) error {
	for _, existing := range f.tickets {
		if existing.WorkspaceID == t.WorkspaceID && existing.BusinessDate == t.BusinessDate && existing.TicketRef == t.TicketRef {
			return cafe.DuplicatePrepConflict()
		}
	}
	f.tickets = append(f.tickets, t)
	return nil
}
func (f *fake) GetPrepTicket(_ context.Context, tid id.ID) (cafe.PrepTicket, error) {
	for _, t := range f.tickets {
		if t.ID == tid {
			return t, nil
		}
	}
	return cafe.PrepTicket{}, cafe.PrepNotFoundError()
}
func (f *fake) ListPrepTickets(_ context.Context, workspaceID id.ID, day string) ([]cafe.PrepTicket, error) {
	out := make([]cafe.PrepTicket, 0)
	for _, t := range f.tickets {
		if t.WorkspaceID == workspaceID && t.BusinessDate == day {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fake) SavePrepTicket(_ context.Context, t cafe.PrepTicket) error {
	for i, existing := range f.tickets {
		if existing.ID == t.ID {
			f.tickets[i] = t
			return nil
		}
	}
	return cafe.PrepNotFoundError()
}

func TestEmptyPrepDayStaysUnavailable(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")}
	svc := cafeprep.New(repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)})
	view, err := svc.Get(t.Context(), gm(wid), "")
	if err != nil || view.Status != "unavailable" || view.TicketCount != 0 || view.LongestOpenMinutes != 0 {
		t.Fatalf("empty prep must not go live: %+v err=%v", view, err)
	}
}

func TestRecordReadyServe(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")}
	clk := clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)}
	svc := cafeprep.New(repo, repo, clk)
	ac := gm(wid)
	opened, err := svc.Record(t.Context(), ac, "", "kitchen", "K-1", "12")
	if err != nil || opened.Status != "live" || opened.WaitingCount != 1 || len(opened.Tickets) != 1 {
		t.Fatalf("open=%+v err=%v", opened, err)
	}
	if _, err := svc.Record(t.Context(), ac, "", "kitchen", "k-1", "12"); err == nil || !errors.Is(err, errx.ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := svc.MarkServed(t.Context(), ac, opened.Tickets[0].ID); err == nil {
		t.Fatal("serve before ready")
	}
	ready, err := svc.MarkReady(t.Context(), ac, opened.Tickets[0].ID)
	if err != nil || ready.WaitingCount != 0 || ready.ReadyCount != 1 {
		t.Fatalf("ready=%+v err=%v", ready, err)
	}
	served, err := svc.MarkServed(t.Context(), ac, opened.Tickets[0].ID)
	if err != nil || served.ServedCount != 1 || served.WaitingCount != 0 || served.Status != "live" {
		t.Fatalf("served=%+v err=%v", served, err)
	}
}

func TestStaffAndHotelRejected(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "UTC")}
	svc := cafeprep.New(repo, repo, clock.System{})
	staff := gm(wid)
	staff.Scope = staffscope.StaffPersonnel
	if _, err := svc.Get(t.Context(), staff, ""); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("staff get: %v", err)
	}
	hotel := gm(wid)
	hotel.Workspace.Kind = tenancy.KindHotel
	if _, err := svc.Record(t.Context(), hotel, "", "kitchen", "k1", ""); err == nil {
		t.Fatal("hotel 422")
	}
}

func gm(wid id.ID) actor.Context {
	uid := id.New()
	return actor.Context{
		Principal: ctxkey.Principal{UserID: uid.String(), WorkspaceID: wid.String(), Role: identity.RoleManager},
		Workspace: tenancy.Workspace{ID: wid, Kind: tenancy.KindCafe, Name: "Duaa Cafe", Timezone: "Asia/Baghdad"},
		Catalog:   cafe.Catalog{},
		Scope:     staffscope.GeneralManager,
	}
}
