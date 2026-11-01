package cafepos_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafepos"
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
	tickets []cafe.Ticket
	close   cafe.POSClose
}

func (f *fake) GetCafeDefinition(_ context.Context, _ id.ID) (cafe.Definition, error) {
	if f.def.WorkspaceID == id.Zero() {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "missing")
	}
	return f.def, nil
}
func (f *fake) CreatePOSTicket(_ context.Context, t cafe.Ticket) error {
	for _, existing := range f.tickets {
		if existing.WorkspaceID == t.WorkspaceID && existing.ExternalRef == t.ExternalRef {
			return cafe.DuplicateTicketConflict()
		}
	}
	f.tickets = append(f.tickets, t)
	return nil
}
func (f *fake) ListPOSTickets(_ context.Context, workspaceID id.ID, day string) ([]cafe.Ticket, error) {
	out := make([]cafe.Ticket, 0)
	for _, t := range f.tickets {
		if t.WorkspaceID == workspaceID && t.BusinessDate == day {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fake) GetPOSClose(_ context.Context, workspaceID id.ID, day string) (cafe.POSClose, error) {
	if f.close.WorkspaceID == workspaceID && f.close.BusinessDate == day {
		return f.close, nil
	}
	return cafe.POSClose{}, errx.NotFound("pos_not_closed", "missing")
}
func (f *fake) CreatePOSClose(_ context.Context, c cafe.POSClose) error {
	f.close = c
	return nil
}

func TestEmptyDayStaysUnavailable(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")}
	svc := cafepos.New(repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)})
	view, err := svc.Get(t.Context(), gm(wid), "")
	if err != nil || view.Status != "unavailable" || view.TicketCount != 0 || view.Net != 0 {
		t.Fatalf("empty POS must not go live: %+v err=%v", view, err)
	}
}

func TestRecordSaleAndReturnThenClose(t *testing.T) {
	wid := id.New()
	repo := &fake{def: namedPOS(wid)}
	svc := cafepos.New(repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)})
	ac := gm(wid)
	if _, err := svc.Close(t.Context(), ac, ""); err == nil {
		t.Fatal("empty close")
	}
	sale, err := svc.Record(t.Context(), ac, "", "S-1", "sale", "cash", 2, 12000, 2000)
	if err != nil || sale.Status != "live" || sale.Net != 10000 || sale.Basket != 10000 {
		t.Fatalf("sale=%+v err=%v", sale, err)
	}
	if _, err := svc.Record(t.Context(), ac, "", "S-1", "sale", "cash", 1, 1, 0); err == nil || !errors.Is(err, errx.ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	ret, err := svc.Record(t.Context(), ac, "", "R-1", "return", "cash", 1, 1000, 0)
	if err != nil || ret.Net != 9000 || ret.Returns != 1000 || ret.ReturnCount != 1 {
		t.Fatalf("return=%+v err=%v", ret, err)
	}
	closed, err := svc.Close(t.Context(), ac, "")
	if err != nil || !closed.Closed || closed.Net != 9000 {
		t.Fatalf("close=%+v err=%v", closed, err)
	}
	if _, err := svc.Record(t.Context(), ac, "", "S-2", "sale", "card", 1, 500, 0); err == nil {
		t.Fatal("closed day must reject new tickets")
	}
}

func TestStaffAndHotelRejected(t *testing.T) {
	wid := id.New()
	repo := &fake{def: namedPOS(wid)}
	svc := cafepos.New(repo, repo, clock.System{})
	staff := gm(wid)
	staff.Scope = staffscope.StaffPersonnel
	if _, err := svc.Get(t.Context(), staff, ""); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("staff get: %v", err)
	}
	hotel := gm(wid)
	hotel.Workspace.Kind = tenancy.KindHotel
	if _, err := svc.Get(t.Context(), hotel, ""); err == nil {
		t.Fatal("hotel 422")
	}
}

func TestNonePOSRejectsRecord(t *testing.T) {
	wid := id.New()
	def := cafe.Seed(wid, "Duaa Cafe", "UTC")
	def.POSStatus = cafe.POSNone
	repo := &fake{def: def}
	svc := cafepos.New(repo, repo, clock.System{})
	if _, err := svc.Record(t.Context(), gm(wid), "", "S-1", "sale", "cash", 1, 100, 0); err == nil {
		t.Fatal("none POS")
	}
}

func namedPOS(wid id.ID) cafe.Definition {
	d := cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")
	d.POSVendor = "Foodics"
	d.POSStatus = cafe.POSNamed
	return d
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
