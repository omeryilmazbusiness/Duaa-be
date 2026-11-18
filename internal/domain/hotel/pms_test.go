package hotel_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestPMSSnapshotCalculationsAndValidation(t *testing.T) {
	now := time.Date(2026, 11, 18, 8, 0, 0, 0, time.UTC)
	snap := hotel.PMSSnapshot{
		WorkspaceID:       id.New(),
		RecordedBy:        id.New(),
		BusinessDate:      "2026-11-18",
		Source:            hotel.SourceOHIPLive,
		Currency:          "usd",
		TotalRooms:        200,
		OutOfOrderRooms:   10,
		OccupiedRooms:     152,
		RoomRevenueMinor:  30400_00, // $30,400.00
		FBRevenueMinor:    4600_00,
		OtherRevenueMinor: 1000_00,
		CapturedAt:        now,
	}
	if err := snap.NormalizeAndValidate(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if snap.AvailableRooms() != 190 {
		t.Fatalf("avail=%d want 190", snap.AvailableRooms())
	}
	if snap.OccupancyBasisPoints() != 8000 {
		t.Fatalf("occ bps=%d want 8000", snap.OccupancyBasisPoints())
	}
	if snap.ADRMinor() != 200_00 {
		t.Fatalf("adr=%d want 20000", snap.ADRMinor())
	}
	if snap.RevPARMinor() != 160_00 {
		t.Fatalf("revpar=%d want 16000", snap.RevPARMinor())
	}
	if snap.TotalRevenueMinor() != 36000_00 {
		t.Fatalf("total=%d want 3600000", snap.TotalRevenueMinor())
	}
}

func TestFreshnessRetainsStaleOnErrorNeverUnavailable(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	if got := hotel.ComputeFreshness(now, nil, hotel.SyncNever); got != hotel.FreshUnavailable {
		t.Fatalf("nil snapshot=%s", got)
	}
	snap := &hotel.PMSSnapshot{CapturedAt: now.Add(-10 * time.Minute)}
	if got := hotel.ComputeFreshness(now, snap, hotel.SyncOK); got != hotel.FreshCurrent {
		t.Fatalf("recent=%s", got)
	}
	if got := hotel.ComputeFreshness(now, snap, hotel.SyncError); got != hotel.FreshStale {
		t.Fatalf("connector error with existing snap must be stale, got %s", got)
	}
	old := &hotel.PMSSnapshot{CapturedAt: now.Add(-2 * time.Hour)}
	if got := hotel.ComputeFreshness(now, old, hotel.SyncOK); got != hotel.FreshDelayed {
		t.Fatalf("2h old=%s", got)
	}
}

func TestNormalizeOHIPHostURL(t *testing.T) {
	if _, err := hotel.NormalizeHostURL("ftp://bad.example.com"); err == nil {
		t.Fatal("ftp must fail")
	}
	if _, err := hotel.NormalizeHostURL("http://external.example.com"); err == nil {
		t.Fatal("non-localhost http must fail")
	}
	got, err := hotel.NormalizeHostURL("https://ohip.oraclehospitality.com/")
	if err != nil || got != "https://ohip.oraclehospitality.com" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
