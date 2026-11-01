package cafehandover_test

import (
	"context"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafehandover"
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
	def    cafe.Definition
	open   *cafe.Handover
	byID   map[id.ID]cafe.Handover
	user   identity.User
	tasks  []cafe.HandoverRef
	recs   []cafe.HandoverRef
	breaks []cafe.HandoverRef
}

func (f *fake) GetCafeDefinition(_ context.Context, _ id.ID) (cafe.Definition, error) {
	if f.def.WorkspaceID == id.Zero() {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "missing")
	}
	return f.def, nil
}
func (f *fake) GetOpenHandover(_ context.Context, _ id.ID) (cafe.Handover, error) {
	if f.open == nil {
		return cafe.Handover{}, errx.NotFound("handover_not_found", "missing")
	}
	return *f.open, nil
}
func (f *fake) GetHandover(_ context.Context, hid id.ID) (cafe.Handover, error) {
	if rec, ok := f.byID[hid]; ok {
		return rec, nil
	}
	return cafe.Handover{}, errx.NotFound("handover_not_found", "missing")
}
func (f *fake) SaveHandover(_ context.Context, rec cafe.Handover) error {
	if f.byID == nil {
		f.byID = map[id.ID]cafe.Handover{}
	}
	f.byID[rec.ID] = rec
	if rec.Open() {
		cp := rec
		f.open = &cp
	} else if f.open != nil && f.open.ID == rec.ID {
		f.open = nil
	}
	return nil
}
func (f *fake) ListHandovers(_ context.Context, _ id.ID) ([]cafe.Handover, error) {
	out := make([]cafe.Handover, 0, len(f.byID))
	for _, rec := range f.byID {
		out = append(out, rec)
	}
	return out, nil
}
func (f *fake) SnapshotOpenWork(_ context.Context, _ id.ID) ([]cafe.HandoverRef, []cafe.HandoverRef, []cafe.HandoverRef, error) {
	return f.tasks, f.recs, f.breaks, nil
}
func (f *fake) GetUser(_ context.Context, _ id.ID) (identity.User, error) {
	return f.user, nil
}

func TestGetDoesNotInventOpenWork(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad"), user: identity.User{FirstName: "Sara"}}
	svc := cafehandover.New(repo, repo, repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)})
	view, err := svc.Get(t.Context(), gm(wid))
	if err != nil || view.Persisted || len(view.Tasks)+len(view.Recoveries)+len(view.Breakdowns) != 0 {
		t.Fatalf("empty pulse must stay empty: %+v err=%v", view, err)
	}
}

func TestOpenFreezesLiveWorkAndAcceptCloses(t *testing.T) {
	wid := id.New()
	task := cafe.HandoverRef{ID: id.New(), Title: "restock milk"}
	repo := &fake{
		def:   cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad"),
		user:  identity.User{FirstName: "Sara", LastName: "GM"},
		tasks: []cafe.HandoverRef{task},
	}
	svc := cafehandover.New(repo, repo, repo, repo, clock.System{})
	ac := gm(wid)
	if _, err := svc.Open(t.Context(), ac, cafehandover.Draft{FromShift: "Night"}); err == nil {
		t.Fatal("unrecorded shift")
	}
	opened, err := svc.Open(t.Context(), ac, cafehandover.Draft{Note: "keys on bar"})
	if err != nil || !opened.Persisted || opened.Status != cafe.HandoverOpen || len(opened.Tasks) != 1 || opened.Tasks[0].Title != "restock milk" {
		t.Fatalf("open=%+v err=%v", opened, err)
	}
	if _, err := svc.Open(t.Context(), ac, cafehandover.Draft{}); err == nil {
		t.Fatal("one open file")
	}
	repo.tasks = nil
	still, err := svc.Get(t.Context(), ac)
	if err != nil || still.ID != opened.ID || len(still.Tasks) != 1 {
		t.Fatalf("snapshot must freeze: %+v err=%v", still, err)
	}
	accepted, err := svc.Accept(t.Context(), ac, opened.ID)
	if err != nil || accepted.Status != cafe.HandoverAccepted || accepted.AcceptedByName != "Sara GM" {
		t.Fatalf("accept=%+v err=%v", accepted, err)
	}
	next, err := svc.Get(t.Context(), ac)
	if err != nil || next.Persisted || len(next.Tasks) != 0 {
		t.Fatalf("after accept, live pulse is empty: %+v err=%v", next, err)
	}
}

func TestHotelCannotOpenHandover(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "UTC")}
	svc := cafehandover.New(repo, repo, repo, repo, clock.System{})
	ac := gm(wid)
	ac.Workspace.Kind = tenancy.KindHotel
	if _, err := svc.Open(t.Context(), ac, cafehandover.Draft{}); err == nil {
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
