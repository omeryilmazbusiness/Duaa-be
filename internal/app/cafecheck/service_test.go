package cafecheck_test

import (
	"context"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafecheck"
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
	def  cafe.Definition
	tpl  cafe.ChecklistTemplate
	runs map[string]cafe.ChecklistRun
	user identity.User
}

func runKey(day string, kind cafe.CheckKind) string { return day + ":" + string(kind) }

func (f *fake) GetCafeDefinition(_ context.Context, _ id.ID) (cafe.Definition, error) {
	if f.def.WorkspaceID == id.Zero() {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "missing")
	}
	return f.def, nil
}
func (f *fake) GetChecklistTemplate(_ context.Context, _ id.ID) (cafe.ChecklistTemplate, error) {
	if f.tpl.WorkspaceID == id.Zero() {
		return cafe.ChecklistTemplate{}, errx.NotFound("checklist_not_found", "missing")
	}
	return f.tpl, nil
}
func (f *fake) SaveChecklistTemplate(_ context.Context, rec cafe.ChecklistTemplate) error {
	f.tpl = rec
	return nil
}
func (f *fake) GetChecklistRun(_ context.Context, _ id.ID, day string, kind cafe.CheckKind) (cafe.ChecklistRun, error) {
	if rec, ok := f.runs[runKey(day, kind)]; ok {
		return rec, nil
	}
	return cafe.ChecklistRun{}, errx.NotFound("checklist_run_not_found", "missing")
}
func (f *fake) SaveChecklistRun(_ context.Context, rec cafe.ChecklistRun) error {
	if f.runs == nil {
		f.runs = map[string]cafe.ChecklistRun{}
	}
	f.runs[runKey(rec.BusinessDate, rec.Kind)] = rec
	return nil
}
func (f *fake) GetUser(_ context.Context, _ id.ID) (identity.User, error) {
	return f.user, nil
}

func TestGetDoesNotInventLines(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad"), user: identity.User{FirstName: "Sara"}}
	svc := cafecheck.New(repo, repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)})
	view, err := svc.Get(t.Context(), gm(wid), "")
	if err != nil || view.HasList || len(view.Items) != 0 || view.Open.Total != 0 {
		t.Fatalf("empty list must stay empty: %+v err=%v", view, err)
	}
}

func TestSaveAndToggleRoundTrip(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad"), user: identity.User{FirstName: "Sara", LastName: "GM"}}
	svc := cafecheck.New(repo, repo, repo, clock.Frozen{T: time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)})
	ac := gm(wid)
	saved, err := svc.SaveTemplate(t.Context(), ac, []cafecheck.DraftItem{
		{Kind: "open", Title: "chairs down"},
		{Kind: "close", Title: "safe locked"},
		{Kind: "maintenance", Title: "photo fridge"},
	})
	if err != nil || !saved.HasList || len(saved.Items) != 2 || saved.Open.Remaining != 1 {
		t.Fatalf("save=%+v err=%v", saved, err)
	}
	openID := ""
	for _, line := range saved.Items {
		if line.Kind == "open" {
			openID = line.ID
		}
	}
	ticked, err := svc.Toggle(t.Context(), ac, saved.BusinessDate, openID)
	if err != nil || ticked.Open.Done != 1 || ticked.Open.Remaining != 0 || ticked.Close.Remaining != 1 {
		t.Fatalf("tick=%+v err=%v", ticked, err)
	}
	if ticked.Items[0].Done && ticked.Items[0].DoneByName != "Sara GM" && ticked.Items[1].DoneByName != "Sara GM" {
		// one of them is the open line
	}
	found := false
	for _, line := range ticked.Items {
		if line.ID == openID && line.Done && line.DoneByName == "Sara GM" {
			found = true
		}
	}
	if !found {
		t.Fatalf("open tick missing name: %+v", ticked.Items)
	}
	undone, err := svc.Toggle(t.Context(), ac, saved.BusinessDate, openID)
	if err != nil || undone.Open.Remaining != 1 {
		t.Fatalf("unmark=%+v err=%v", undone, err)
	}
}

func TestUnknownLineRejected(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "UTC")}
	svc := cafecheck.New(repo, repo, repo, clock.System{})
	if _, err := svc.Toggle(t.Context(), gm(wid), "2026-11-02", id.New().String()); err == nil {
		t.Fatal("unrecorded line")
	}
}

func TestHotelRejected(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "UTC")}
	svc := cafecheck.New(repo, repo, repo, clock.System{})
	ac := gm(wid)
	ac.Workspace.Kind = tenancy.KindHotel
	if _, err := svc.Get(t.Context(), ac, ""); err == nil {
		t.Fatal("hotel 422")
	}
}

func TestFloorCannotEditTemplate(t *testing.T) {
	wid := id.New()
	repo := &fake{def: cafe.Seed(wid, "Duaa Cafe", "UTC")}
	svc := cafecheck.New(repo, repo, repo, clock.System{})
	ac := gm(wid)
	ac.Scope = staffscope.FloorOps
	if _, err := svc.SaveTemplate(t.Context(), ac, []cafecheck.DraftItem{{Kind: "open", Title: "lights"}}); err == nil {
		t.Fatal("floor must not write the list")
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
