package hotelpms_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/hotelpms"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type fakeSpaces struct {
	ws      map[id.ID]tenancy.Workspace
	updates int
}

func (f *fakeSpaces) GetWorkspace(_ context.Context, wid id.ID) (tenancy.Workspace, error) {
	w, ok := f.ws[wid]
	if !ok {
		return tenancy.Workspace{}, errx.NotFound("workspace_not_found", "workspace not found")
	}
	return w, nil
}

func (f *fakeSpaces) UpdateWorkspace(_ context.Context, w tenancy.Workspace) error {
	f.ws[w.ID] = w
	f.updates++
	return nil
}

func TestHotelProfileSeedsFromWorkspaceAndLatestSnapshot(t *testing.T) {
	wid := id.New()
	repo := newFakeRepo()
	svc := hotelpms.New(repo, nil, nil, clock.Frozen{T: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)})
	ac := hotelGM(wid)

	if _, err := svc.ImportSnapshot(t.Context(), ac, hotelpms.SnapshotInput{
		BusinessDate: "2026-11-18", Source: "manual", ExternalBatchRef: "m-1", Currency: "IQD", TotalRooms: 180, OccupiedRooms: 90,
	}); err != nil {
		t.Fatal(err)
	}
	view, err := svc.GetProfile(t.Context(), ac)
	if err != nil {
		t.Fatal(err)
	}
	if view.Persisted || !view.CanWrite || view.DisplayName != "Sheraton Hotel" || view.TotalRooms != 180 || view.Currency != "IQD" || view.Timezone != "Asia/Baghdad" {
		t.Fatalf("seed view=%+v", view)
	}
}

func TestHotelProfileSaveRenamesWorkspace(t *testing.T) {
	wid := id.New()
	repo := newFakeRepo()
	ac := hotelGM(wid)
	spaces := &fakeSpaces{ws: map[id.ID]tenancy.Workspace{wid: ac.Workspace}}
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	svc := hotelpms.New(repo, nil, nil, clock.Frozen{T: now}).WithWorkspaces(spaces)

	view, err := svc.SaveProfile(t.Context(), ac, hotelpms.SaveProfileInput{
		DisplayName: "  Sheraton  Baghdad ", TotalRooms: 250, Currency: "usd", StarRating: 5, City: "Baghdad",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.Persisted || view.DisplayName != "Sheraton Baghdad" || view.Currency != "USD" || view.Timezone != "Asia/Baghdad" || view.UpdatedAt == "" {
		t.Fatalf("saved view=%+v", view)
	}
	if spaces.updates != 1 || spaces.ws[wid].Name != "Sheraton Baghdad" {
		t.Fatalf("workspace not renamed: %+v (updates=%d)", spaces.ws[wid], spaces.updates)
	}

	again, err := svc.GetProfile(t.Context(), ac)
	if err != nil || !again.Persisted || again.TotalRooms != 250 || again.StarRating != 5 {
		t.Fatalf("reload=%+v err=%v", again, err)
	}

	dash, err := svc.GetDashboard(t.Context(), ac, "2026-11-18")
	if err != nil || dash.Profile.DisplayName != "Sheraton Baghdad" {
		t.Fatalf("dashboard must embed the profile, got %+v err=%v", dash.Profile, err)
	}
}

func TestHotelProfileRules(t *testing.T) {
	wid := id.New()
	svc := hotelpms.New(newFakeRepo(), nil, nil, clock.System{})

	dept := hotelGM(wid)
	dept.Scope = staffscope.DepartmentManager
	if _, err := svc.SaveProfile(t.Context(), dept, hotelpms.SaveProfileInput{DisplayName: "X"}); !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("department manager must not edit hotel info, got %v", err)
	}
	if view, err := svc.GetProfile(t.Context(), dept); err != nil || view.CanWrite {
		t.Fatalf("department manager reads without write, got %+v err=%v", view, err)
	}

	staff := hotelGM(wid)
	staff.Scope = staffscope.StaffPersonnel
	if _, err := svc.GetProfile(t.Context(), staff); !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("staff must not read hotel info, got %v", err)
	}

	gm := hotelGM(wid)
	if _, err := svc.SaveProfile(t.Context(), gm, hotelpms.SaveProfileInput{DisplayName: "", TotalRooms: 10}); !errors.Is(err, errx.ErrInvalid) {
		t.Fatalf("empty name must be invalid, got %v", err)
	}
	if _, err := svc.SaveProfile(t.Context(), gm, hotelpms.SaveProfileInput{DisplayName: "X", Timezone: "Nowhere/City"}); !errors.Is(err, errx.ErrInvalid) {
		t.Fatalf("bad timezone must be invalid, got %v", err)
	}

	cafe := hotelGM(wid)
	cafe.Workspace.Kind = tenancy.KindCafe
	if _, err := svc.SaveProfile(t.Context(), cafe, hotelpms.SaveProfileInput{DisplayName: "X"}); !errors.Is(err, errx.ErrInvalid) {
		t.Fatalf("cafe workspace must be rejected, got %v", err)
	}
}
