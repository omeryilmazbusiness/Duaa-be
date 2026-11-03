package cafewaste_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafewaste"
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
	tickets []cafe.WasteTicket
	book    cafe.RecipeBook
}

func (f *fake) GetCafeDefinition(_ context.Context, _ id.ID) (cafe.Definition, error) {
	if f.def.WorkspaceID == id.Zero() {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "missing")
	}
	return f.def, nil
}
func (f *fake) CreateWasteTicket(_ context.Context, t cafe.WasteTicket) error {
	f.tickets = append(f.tickets, t)
	return nil
}
func (f *fake) ListWasteTickets(_ context.Context, workspaceID id.ID, day string) ([]cafe.WasteTicket, error) {
	out := make([]cafe.WasteTicket, 0)
	for _, t := range f.tickets {
		if t.WorkspaceID == workspaceID && t.BusinessDate == day {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fake) GetRecipeBook(_ context.Context, workspaceID id.ID) (cafe.RecipeBook, error) {
	if f.book.WorkspaceID == id.Zero() {
		return cafe.RecipeBook{}, cafe.RecipeBookNotFoundError()
	}
	if f.book.WorkspaceID != workspaceID {
		return cafe.RecipeBook{}, cafe.RecipeBookNotFoundError()
	}
	return f.book, nil
}
func (f *fake) SaveRecipeBook(_ context.Context, rec cafe.RecipeBook) error {
	f.book = rec
	return nil
}

func TestEmptyWasteDayStaysUnavailable(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")}
	svc := cafewaste.New(repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)})
	view, err := svc.Get(t.Context(), gm(wid), "")
	if err != nil || view.Status != "unavailable" || view.TicketCount != 0 || view.Cost != 0 || view.Grams != 0 {
		t.Fatalf("empty waste must not go live: %+v err=%v", view, err)
	}
}

func TestRecordWasteGoesLiveWithoutPOS(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")}
	svc := cafewaste.New(repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)})
	ac := gm(wid)
	view, err := svc.Record(t.Context(), ac, "", "kitchen", "milk", "spoilage", "", "", 500, 8000)
	if err != nil || view.Status != "live" || view.TicketCount != 1 || view.Cost != 8000 || view.Grams != 500 {
		t.Fatalf("record=%+v err=%v", view, err)
	}
	if _, err := svc.Record(t.Context(), ac, "", "kitchen", "milk", "spoilage", "", "", 100, 0); err == nil {
		t.Fatal("zero cost")
	}
}

func TestRecipeBookEmptyValidAndUnknownRecipeRejected(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")}
	svc := cafewaste.New(repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)})
	ac := gm(wid)
	empty, err := svc.SaveRecipes(t.Context(), ac, nil)
	if err != nil || len(empty.Recipes) != 0 || empty.Status != "unavailable" {
		t.Fatalf("empty recipes=%+v err=%v", empty, err)
	}
	saved, err := svc.SaveRecipes(t.Context(), ac, []cafewaste.RecipeDraft{{
		Name: "Latte", Station: "bar", Lines: []cafewaste.RecipeLineView{{Title: "milk", Grams: 180}},
	}})
	if err != nil || len(saved.Recipes) != 1 {
		t.Fatalf("save=%+v err=%v", saved, err)
	}
	if _, err := svc.Record(t.Context(), ac, "", "bar", "latte", "prep", "", id.New().String(), 180, 2000); err == nil {
		t.Fatal("unknown recipe")
	}
	live, err := svc.Record(t.Context(), ac, "", "bar", "latte", "prep", "", saved.Recipes[0].ID, 180, 2000)
	if err != nil || live.Status != "live" || live.Tickets[0].RecipeID != saved.Recipes[0].ID {
		t.Fatalf("linked=%+v err=%v", live, err)
	}
}

func TestStaffAndHotelRejected(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "UTC")}
	svc := cafewaste.New(repo, repo, clock.System{})
	staff := gm(wid)
	staff.Scope = staffscope.StaffPersonnel
	if _, err := svc.Get(t.Context(), staff, ""); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("staff get: %v", err)
	}
	floor := gm(wid)
	floor.Scope = staffscope.FloorOps
	if _, err := svc.SaveRecipes(t.Context(), floor, nil); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("floor recipes: %v", err)
	}
	hotel := gm(wid)
	hotel.Workspace.Kind = tenancy.KindHotel
	if _, err := svc.Record(t.Context(), hotel, "", "kitchen", "bread", "", "", "", 100, 500); err == nil {
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
