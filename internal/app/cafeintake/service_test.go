package cafeintake_test

import (
	"context"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafeintake"
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
	byDay   map[string]cafe.Intake
	user    identity.User
}

func (f *fake) GetCafeDefinition(_ context.Context, _ id.ID) (cafe.Definition, error) {
	if f.def.WorkspaceID == id.Zero() {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "missing")
	}
	return f.def, nil
}
func (f *fake) GetIntake(_ context.Context, _ id.ID, day string) (cafe.Intake, error) {
	if rec, ok := f.byDay[day]; ok {
		return rec, nil
	}
	return cafe.Intake{}, errx.NotFound("intake_not_found", "missing")
}
func (f *fake) SaveIntake(_ context.Context, rec cafe.Intake) error {
	if f.byDay == nil {
		f.byDay = map[string]cafe.Intake{}
	}
	f.byDay[rec.BusinessDate] = rec
	return nil
}
func (f *fake) ListIntakes(_ context.Context, _ id.ID) ([]cafe.Intake, error) {
	out := make([]cafe.Intake, 0, len(f.byDay))
	for _, rec := range f.byDay {
		out = append(out, rec)
	}
	return out, nil
}
func (f *fake) GetUser(_ context.Context, _ id.ID) (identity.User, error) {
	return f.user, nil
}

func TestGetDoesNotInventAClose(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad"), user: identity.User{FirstName: "Sara"}}
	svc := cafeintake.New(repo, repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)})
	view, err := svc.Get(t.Context(), gm(wid), "")
	if err != nil || view.Persisted || view.BusinessDate != "2026-11-02" || view.CloseNote != "" {
		t.Fatalf("empty day must stay empty: %+v err=%v", view, err)
	}
}

func TestSaveUpsertsTheSameDayAndRejectsInventedShift(t *testing.T) {
	wid := id.New()
	def := cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")
	repo := &fake{def: def, user: identity.User{FirstName: "Sara", LastName: "GM"}}
	svc := cafeintake.New(repo, repo, repo, clock.System{})
	ac := gm(wid)
	_, err := svc.Save(t.Context(), ac, cafeintake.Draft{ShiftName: "Night", WasteNote: "bread"})
	if err == nil {
		t.Fatal("unrecorded shift")
	}
	first, err := svc.Save(t.Context(), ac, cafeintake.Draft{CloseNote: "quiet close"})
	if err != nil || !first.Persisted || first.AuthorName != "Sara GM" {
		t.Fatalf("save=%+v err=%v", first, err)
	}
	again, err := svc.Save(t.Context(), ac, cafeintake.Draft{BusinessDate: first.BusinessDate, CloseNote: "quiet close", IncidentNote: "glass broke"})
	if err != nil || again.ID != first.ID || again.IncidentNote != "glass broke" {
		t.Fatalf("upsert=%+v err=%v", again, err)
	}
}

func TestHotelCannotWriteIntake(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "UTC")}
	svc := cafeintake.New(repo, repo, repo, clock.System{})
	ac := gm(wid)
	ac.Workspace.Kind = tenancy.KindHotel
	if _, err := svc.Save(t.Context(), ac, cafeintake.Draft{CloseNote: "ok"}); err == nil {
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
