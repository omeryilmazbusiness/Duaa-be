package hotelpms_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/hotelpms"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/platform/secretbox"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type fakeRepo struct {
	cfgs     map[id.ID]hotel.OHIPConfig
	snaps    map[string]hotel.PMSSnapshot
	profiles map[id.ID]hotel.Profile
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{cfgs: map[id.ID]hotel.OHIPConfig{}, snaps: map[string]hotel.PMSSnapshot{}, profiles: map[id.ID]hotel.Profile{}}
}

func (f *fakeRepo) GetHotelProfile(_ context.Context, wid id.ID) (hotel.Profile, error) {
	p, ok := f.profiles[wid]
	if !ok {
		return hotel.Profile{}, hotel.ProfileNotFoundError()
	}
	return p, nil
}
func (f *fakeRepo) SaveHotelProfile(_ context.Context, p hotel.Profile) error {
	f.profiles[p.WorkspaceID] = p
	return nil
}

func (f *fakeRepo) GetOHIPConfig(_ context.Context, wid id.ID) (hotel.OHIPConfig, error) {
	c, ok := f.cfgs[wid]
	if !ok {
		return hotel.OHIPConfig{}, hotel.OHIPConfigNotFoundError()
	}
	return c, nil
}
func (f *fakeRepo) SaveOHIPConfig(_ context.Context, cfg hotel.OHIPConfig) error {
	f.cfgs[cfg.WorkspaceID] = cfg
	return nil
}
func (f *fakeRepo) GetPMSSnapshot(_ context.Context, wid id.ID, day string) (hotel.PMSSnapshot, error) {
	s, ok := f.snaps[wid.String()+":"+day]
	if !ok {
		return hotel.PMSSnapshot{}, hotel.PMSSnapshotNotFoundError()
	}
	return s, nil
}
func (f *fakeRepo) SavePMSSnapshot(_ context.Context, s hotel.PMSSnapshot) error {
	f.snaps[s.WorkspaceID.String()+":"+s.BusinessDate] = s
	return nil
}
func (f *fakeRepo) ListPMSSnapshots(_ context.Context, wid id.ID, _ int) ([]hotel.PMSSnapshot, error) {
	var out []hotel.PMSSnapshot
	for _, s := range f.snaps {
		if s.WorkspaceID == wid {
			out = append(out, s)
		}
	}
	return out, nil
}

type fakeOHIP struct {
	snap hotel.PMSSnapshot
	err  error
}

func (f *fakeOHIP) FetchDay(_ context.Context, _ hotelpms.OHIPFetchRequest) (hotel.PMSSnapshot, error) {
	return f.snap, f.err
}

func TestHotelPMSComparisonAndStaleFallback(t *testing.T) {
	wid := id.New()
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	box, err := secretbox.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	client := &fakeOHIP{}
	svc := hotelpms.New(repo, box, client, clock.Frozen{T: now})
	ac := hotelGM(wid)

	empty, err := svc.GetDashboard(t.Context(), ac, "2026-11-18")
	if err != nil || empty.Freshness != "unavailable" || empty.Today != nil {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}

	// Import yesterday + today OPERA CSV.
	_, err = svc.ImportSnapshot(t.Context(), ac, hotelpms.SnapshotInput{
		BusinessDate:     "2026-11-17",
		Source:           "opera_export",
		ExternalBatchRef: "op-yday-1",
		Currency:         "USD",
		TotalRooms:       100,
		OccupiedRooms:    70,
		RoomRevenueMinor: 14000_00,
		FBRevenueMinor:   2000_00,
	})
	if err != nil {
		t.Fatal(err)
	}
	csv := "business_date,currency,total_rooms,occupied_rooms,out_of_order_rooms,room_revenue,fb_revenue,arrivals,departures\n2026-11-18,USD,100,80,0,18000.00,2000.00,24,18\n"
	today, err := svc.ImportOPERAFile(t.Context(), ac, "2026-11-18", "op-today-1", csv)
	if err != nil {
		t.Fatal(err)
	}
	if today.Freshness != "current" || today.Today == nil || today.Today.TotalRevenueMinor != 20000_00 {
		t.Fatalf("today=%+v", today)
	}
	if today.Comparison == nil || !today.Comparison.HasYesterday || today.Comparison.RevenueDeltaMinor != 4000_00 {
		t.Fatalf("comparison=%+v", today.Comparison)
	}

	// Replay with same ExternalBatchRef must report Replayed=true.
	replay, err := svc.ImportOPERAFile(t.Context(), ac, "2026-11-18", "op-today-1", csv)
	if err != nil || !replay.Replayed {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}

	// Configure OHIP and simulate outage: must keep last valid values with stale freshness (AT-10).
	cfgView, err := svc.SaveOHIPConfig(t.Context(), ac, hotelpms.SaveOHIPInput{
		Enabled: true, HostURL: "https://ohip.example.com", HotelID: "SHERATON",
		ClientID: "cid", ClientSecret: "sec-1234", AppKey: "app-5678", SyncMode: "ohip_api",
	})
	if err != nil || !cfgView.Configured || cfgView.SecretHint != "••••1234" {
		t.Fatalf("cfg=%+v err=%v", cfgView, err)
	}
	client.err = errors.New("gateway timeout")
	staleView, err := svc.SyncOHIP(t.Context(), ac, "2026-11-18")
	if err != nil {
		t.Fatal(err)
	}
	if staleView.Freshness != "stale" || staleView.Today == nil || staleView.Today.TotalRevenueMinor != 20000_00 {
		t.Fatalf("AT-10 failed: must retain last valid revenue as stale, got %+v", staleView)
	}
}

func TestHotelPMSRejectsCafeWorkspace(t *testing.T) {
	svc := hotelpms.New(newFakeRepo(), nil, nil, clock.System{})
	ac := hotelGM(id.New())
	ac.Workspace.Kind = tenancy.KindCafe
	if _, err := svc.GetDashboard(t.Context(), ac, ""); err == nil || !errors.Is(err, errx.ErrInvalid) {
		t.Fatalf("expected invalid for cafe, got %v", err)
	}
}

func hotelGM(wid id.ID) actor.Context {
	uid := id.New()
	return actor.Context{
		Principal: ctxkey.Principal{UserID: uid.String(), WorkspaceID: wid.String(), Role: identity.RoleManager},
		Workspace: tenancy.Workspace{ID: wid, Kind: tenancy.KindHotel, Name: "Sheraton Hotel", Timezone: "Asia/Baghdad"},
		Catalog:   hotel.Catalog{},
		Scope:     staffscope.GeneralManager,
	}
}
