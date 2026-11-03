package cafestock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafestock"
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
	def   cafe.Definition
	book  cafe.StockCatalog
	adjs  []cafe.StockAdjustment
}

func (f *fake) GetCafeDefinition(_ context.Context, _ id.ID) (cafe.Definition, error) {
	if f.def.WorkspaceID == id.Zero() {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "missing")
	}
	return f.def, nil
}
func (f *fake) GetStockCatalog(_ context.Context, workspaceID id.ID) (cafe.StockCatalog, error) {
	if f.book.WorkspaceID == id.Zero() {
		return cafe.StockCatalog{}, cafe.StockCatalogNotFoundError()
	}
	if f.book.WorkspaceID != workspaceID {
		return cafe.StockCatalog{}, cafe.StockCatalogNotFoundError()
	}
	return f.book, nil
}
func (f *fake) SaveStockCatalog(_ context.Context, rec cafe.StockCatalog) error {
	f.book = rec
	return nil
}
func (f *fake) CreateStockAdjustment(_ context.Context, adj cafe.StockAdjustment) error {
	f.adjs = append(f.adjs, adj)
	return nil
}
func (f *fake) ListStockAdjustments(_ context.Context, workspaceID id.ID, day string) ([]cafe.StockAdjustment, error) {
	out := make([]cafe.StockAdjustment, 0)
	for _, adj := range f.adjs {
		if adj.WorkspaceID == workspaceID && adj.BusinessDate == day {
			out = append(out, adj)
		}
	}
	return out, nil
}

func TestEmptyStockStaysUnavailable(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")}
	svc := cafestock.New(repo, repo, clock.Frozen{T: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)})
	view, err := svc.Get(t.Context(), gm(wid), "")
	if err != nil || view.Status != "unavailable" || view.ItemCount != 0 || len(view.Items) != 0 {
		t.Fatalf("empty stock must stay unavailable: %+v err=%v", view, err)
	}
}

func TestCatalogAndCountGoLive(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")}
	svc := cafestock.New(repo, repo, clock.Frozen{T: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)})
	ac := gm(wid)

	saved, err := svc.SaveCatalog(t.Context(), ac, []cafestock.ItemDraft{{
		Name: "Whole milk", Station: "bar", Unit: "ml", Par: 5000, Active: true,
	}})
	if err != nil || saved.Status != "live" || saved.ItemCount != 1 || saved.Items[0].OnHand != 0 {
		t.Fatalf("catalog=%+v err=%v", saved, err)
	}
	itemID := saved.Items[0].ID
	onHand := int64(4200)
	counted, err := svc.Adjust(t.Context(), ac, "", itemID, "count", "morning", &onHand, nil)
	if err != nil || counted.Items[0].OnHand != 4200 || counted.Items[0].Level != 84 || counted.LowCount != 1 {
		t.Fatalf("count=%+v err=%v", counted, err)
	}
	if len(repo.adjs) != 1 || repo.adjs[0].Delta != 4200 || repo.adjs[0].OnHandAfter != 4200 {
		t.Fatalf("adj=%+v", repo.adjs)
	}
	delta := int64(-200)
	used, err := svc.Adjust(t.Context(), ac, "", itemID, "use", "", nil, &delta)
	if err != nil || used.Items[0].OnHand != 4000 {
		t.Fatalf("use=%+v err=%v", used, err)
	}
}

func TestStaffAndHotelRejected(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "UTC")}
	svc := cafestock.New(repo, repo, clock.System{})
	staff := gm(wid)
	staff.Scope = staffscope.StaffPersonnel
	if _, err := svc.Get(t.Context(), staff, ""); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("staff get: %v", err)
	}
	floor := gm(wid)
	floor.Scope = staffscope.FloorOps
	if _, err := svc.SaveCatalog(t.Context(), floor, nil); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("floor catalog: %v", err)
	}
	hotel := gm(wid)
	hotel.Workspace.Kind = tenancy.KindHotel
	if _, err := svc.Get(t.Context(), hotel, ""); err == nil {
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
