package httpadapter_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/bootstrap"
)

func TestPortfolioHotelsOHIPAndIsolation(t *testing.T) {
	h := newHarness(t)
	hotels, err := bootstrap.SeedPortfolioHotels(t.Context(), h.store, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	login := post(h.handler, "/v1/auth/login", map[string]string{
		"email": h.seed.GMEmail, "password": h.seed.GMPassword,
	}, "")
	if login.Code != 200 {
		t.Fatalf("login %d %s", login.Code, login.Body.String())
	}
	var wrap wrapLogin
	decode(t, login, &wrap)
	if wrap.Token == "" {
		t.Fatal("expected direct token on primary workspace")
	}

	wsList := get(h.handler, "/v1/me/workspaces", wrap.Token)
	if wsList.Code != 200 {
		t.Fatalf("workspaces %d %s", wsList.Code, wsList.Body.String())
	}
	var wsOut struct {
		Workspaces []map[string]any `json:"workspaces"`
	}
	decode(t, wsList, &wsOut)
	if len(wsOut.Workspaces) != 3 {
		t.Fatalf("expected 3 seeded workspaces (cafe + 2 hotels), got %d", len(wsOut.Workspaces))
	}

	// Cafe token must NOT reach Sheraton Hotel routes (workspace isolation AT-01).
	sheratonPath := "/v1/workspaces/" + hotels.SheratonID.String() + "/hotel/pms"
	if get(h.handler, sheratonPath, wrap.Token).Code != 403 {
		t.Fatal("cafe-scoped token must be rejected on hotel route")
	}

	// Switch to Sheraton Hotel cleanly.
	switched := post(h.handler, "/v1/me/workspaces/"+hotels.SheratonID.String()+"/select", nil, wrap.Token)
	if switched.Code != 200 {
		t.Fatalf("switch %d %s", switched.Code, switched.Body.String())
	}
	var hWrap wrapLogin
	decode(t, switched, &hWrap)
	if hWrap.Session.Workspace.Kind != "hotel" {
		t.Fatalf("expected hotel workspace, got %s", hWrap.Session.Workspace.Kind)
	}

	// Unwired hotel stays unavailable.
	empty := get(h.handler, sheratonPath, hWrap.Token)
	if empty.Code != 200 {
		t.Fatalf("empty hotel pms %d %s", empty.Code, empty.Body.String())
	}
	var emptyView map[string]any
	decode(t, empty, &emptyView)
	if emptyView["freshness"] != "unavailable" {
		t.Fatalf("expected unavailable, got %v", emptyView["freshness"])
	}

	// Import yesterday & today OPERA snapshot.
	today := time.Now().UTC().Format("2006-01-02")
	yday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if post(h.handler, sheratonPath+"/import", map[string]any{
		"businessDate":     yday,
		"source":           "opera_export",
		"externalBatchRef": "sh-yday",
		"currency":         "USD",
		"totalRooms":       180,
		"occupiedRooms":    126,
		"roomRevenueMinor": 25200_00,
	}, hWrap.Token).Code != 200 {
		t.Fatal("import yesterday failed")
	}
	imp := post(h.handler, sheratonPath+"/import", map[string]any{
		"businessDate":     today,
		"source":           "opera_export",
		"externalBatchRef": "sh-today",
		"currency":         "USD",
		"totalRooms":       180,
		"occupiedRooms":    144,
		"roomRevenueMinor": 28800_00,
		"fbRevenueMinor":   3200_00,
		"arrivalsCount":    32,
		"departuresCount":  21,
	}, hWrap.Token)
	if imp.Code != 200 {
		t.Fatalf("import today %d %s", imp.Code, imp.Body.String())
	}
	var live map[string]any
	decode(t, imp, &live)
	if live["freshness"] != "current" {
		t.Fatalf("expected current freshness, got %v", live["freshness"])
	}
}
