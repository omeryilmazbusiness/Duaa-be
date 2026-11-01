package tenancy_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/supply"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestRegistryHoldsAllVerticals(t *testing.T) {
	reg := tenancy.NewRegistry(cafe.Catalog{}, hotel.Catalog{}, supply.Catalog{})
	for _, kind := range []tenancy.Kind{tenancy.KindCafe, tenancy.KindHotel, tenancy.KindSupply} {
		if _, err := reg.Catalog(kind); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
}

func TestNewWorkspaceRejectsUnknownKind(t *testing.T) {
	_, err := tenancy.NewWorkspace(id.New(), "bakery", "X", "x", "UTC", time.Now().UTC())
	if err == nil {
		t.Fatal("expected invalid kind")
	}
}

func TestSlugNormalization(t *testing.T) {
	got, err := tenancy.NormalizeSlug(" Duaa Cafe ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "duaa-cafe" {
		t.Fatalf("got %s", got)
	}
}

func TestCafeSignalsOmitHotelReservations(t *testing.T) {
	signals := cafe.Catalog{}.ExecutiveSignals()
	if !tenancy.ContainsSignal(signals, tenancy.SignalLateStaff) {
		t.Fatal("cafe morning OS is staff work")
	}
	if !tenancy.ContainsSignal(signals, tenancy.SignalCustomerVoice) {
		t.Fatal("cafe catalogues QR voice even before ingest exists")
	}
	if tenancy.ContainsSignal(signals, tenancy.SignalReservations) {
		t.Fatal("cafe must not pull hotel reservation signals")
	}
	if !tenancy.ContainsSignal(signals, tenancy.SignalCashVariance) {
		t.Fatal("cafe catalogues cash variance once a till close exists")
	}
	mods := cafe.Catalog{}.DashboardModules()
	if !containsModule(mods, tenancy.ModuleCash) || !containsModule(mods, tenancy.ModuleSales) {
		t.Fatal("cafe dashboard opts into cash and honest POS sales")
	}
}

func containsModule(list []tenancy.DashboardModule, want tenancy.DashboardModule) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestCafeWorkspaceIsolatedFromHotelKind(t *testing.T) {
	ws, err := tenancy.NewWorkspace(id.New(), tenancy.KindCafe, "Duaa Cafe", "duaa-cafe", "Asia/Baghdad", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if ws.Kind != tenancy.KindCafe {
		t.Fatalf("kind=%s", ws.Kind)
	}
}
