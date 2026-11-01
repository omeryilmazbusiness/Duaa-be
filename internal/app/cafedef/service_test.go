package cafedef_test

import (
	"context"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafedef"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type fakeRepo struct {
	defs map[id.ID]cafe.Definition
	ws   tenancy.Workspace
}

func (f *fakeRepo) GetCafeDefinition(_ context.Context, workspaceID id.ID) (cafe.Definition, error) {
	d, ok := f.defs[workspaceID]
	if !ok {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "cafe definition not found")
	}
	return d.Clone(), nil
}

func (f *fakeRepo) SaveCafeDefinition(_ context.Context, def cafe.Definition) error {
	if f.defs == nil {
		f.defs = map[id.ID]cafe.Definition{}
	}
	f.defs[def.WorkspaceID] = def.Clone()
	return nil
}

func (f *fakeRepo) GetWorkspace(_ context.Context, wid id.ID) (tenancy.Workspace, error) {
	if f.ws.ID != wid {
		return tenancy.Workspace{}, errx.NotFound("workspace_not_found", "workspace not found")
	}
	return f.ws, nil
}

func (f *fakeRepo) UpdateWorkspace(_ context.Context, w tenancy.Workspace) error {
	f.ws = w
	return nil
}

func TestGetSeedsWithoutWriting(t *testing.T) {
	ac := gmActor()
	repo := &fakeRepo{ws: ac.Workspace}
	svc := cafedef.New(repo, repo, clock.System{})
	view, err := svc.Get(t.Context(), ac)
	if err != nil {
		t.Fatal(err)
	}
	if view.Persisted || view.Definition.DisplayName != "Duaa Cafe" || view.Completeness.ReadyForReviews {
		t.Fatalf("seed view = %+v", view)
	}
	if _, err := repo.GetCafeDefinition(t.Context(), ac.Workspace.ID); err == nil {
		t.Fatal("GET must not persist a seed")
	}
}

func TestSaveWritesAndSyncsWorkspace(t *testing.T) {
	ac := gmActor()
	repo := &fakeRepo{ws: ac.Workspace}
	svc := cafedef.New(repo, repo, clock.System{})
	view, err := svc.Save(t.Context(), ac, cafedef.SaveInput{
		DisplayName: "Duaa Cafe Karada",
		Currency:    "IQD",
		Timezone:    "Asia/Baghdad",
		POSStatus:   "none",
		Branches:    []cafe.Branch{{Name: "Karada", Active: true}},
		QRSites:     []cafe.QRSite{{Label: "Garden 4", Area: cafe.AreaGarden, Active: true}},
		SLA:         cafe.SLA{AckMinutes: 15, ResolveMinutes: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.Persisted || !view.Completeness.ReadyForReviews {
		t.Fatalf("saved = %+v", view)
	}
	if repo.ws.Name != "Duaa Cafe Karada" {
		t.Fatalf("workspace = %+v", repo.ws)
	}
}

func TestHotelAndStaffAreRejected(t *testing.T) {
	repo := &fakeRepo{}
	svc := cafedef.New(repo, repo, clock.System{})
	hotel := gmActor()
	hotel.Workspace.Kind = tenancy.KindHotel
	if _, err := svc.Get(t.Context(), hotel); err == nil {
		t.Fatal("hotel must 422")
	}
	staff := gmActor()
	staff.Scope = staffscope.StaffPersonnel
	if _, err := svc.Get(t.Context(), staff); err == nil {
		t.Fatal("staff must not read")
	}
	mgr := gmActor()
	mgr.Scope = staffscope.DepartmentManager
	if _, err := svc.Save(t.Context(), mgr, cafedef.SaveInput{DisplayName: "X"}); err == nil {
		t.Fatal("manager must not write")
	}
}

func gmActor() actor.Context {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	ws := tenancy.Workspace{
		ID:             id.MustParse("22222222-2222-4222-8222-222222222222"),
		OrganizationID: id.MustParse("11111111-1111-4111-8111-111111111111"),
		Kind:           tenancy.KindCafe,
		Name:           "Duaa Cafe",
		Slug:           "duaa-cafe",
		Timezone:       "Asia/Baghdad",
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return actor.Context{
		Principal: ctxkey.Principal{UserID: "33333333-3333-4333-8333-333333333333", WorkspaceID: ws.ID.String(), Role: identity.RoleManager},
		Workspace: ws,
		Catalog:   cafe.Catalog{},
		Scope:     staffscope.GeneralManager,
	}
}
