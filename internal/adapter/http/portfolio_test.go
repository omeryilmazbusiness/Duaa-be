package httpadapter_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/bootstrap"
)

func TestExecutivePortfolioEndpointRespectsMembershipScope(t *testing.T) {
	h := newHarness(t)
	hotels, err := bootstrap.SeedPortfolioHotels(t.Context(), h.store, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	login := post(h.handler, "/v1/auth/login", map[string]string{
		"email": h.seed.GMEmail, "password": h.seed.GMPassword,
	}, "")
	var gm wrapLogin
	decode(t, login, &gm)

	res := get(h.handler, "/v1/executive/portfolio", gm.Token)
	if res.Code != 200 {
		t.Fatalf("portfolio %d %s", res.Code, res.Body.String())
	}
	var pv struct {
		WorkspaceCount int `json:"workspaceCount"`
		Hotels         struct {
			Count int `json:"count"`
		} `json:"hotels"`
		Cafes struct {
			Count int `json:"count"`
		} `json:"cafes"`
	}
	decode(t, res, &pv)
	if pv.WorkspaceCount != 3 || pv.Hotels.Count != 2 || pv.Cafes.Count != 1 {
		t.Fatalf("unexpected portfolio=%+v", pv)
	}

	// Cafe-only department manager must NOT see Sheraton/Al Shohada in portfolio (AT-01).
	_ = post(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/staff/department-managers", map[string]string{
		"firstName": "Layla", "lastName": "Kitchen",
		"email": "layla.port@cafe.test", "password": "Kitchen123",
		"staffDepartment": "KITCHEN",
	}, gm.Token)
	mgrLogin := post(h.handler, "/v1/auth/login", map[string]string{
		"email": "layla.port@cafe.test", "password": "Kitchen123",
	}, "")
	var mgr wrapLogin
	decode(t, mgrLogin, &mgr)
	mgrPort := get(h.handler, "/v1/executive/portfolio", mgr.Token)
	if mgrPort.Code != 200 {
		t.Fatalf("mgr portfolio %d", mgrPort.Code)
	}
	var mpv struct {
		WorkspaceCount int `json:"workspaceCount"`
		Hotels         struct {
			Count int `json:"count"`
		} `json:"hotels"`
	}
	decode(t, mgrPort, &mpv)
	if mpv.WorkspaceCount != 1 || mpv.Hotels.Count != 0 {
		t.Fatalf("cafe dept mgr must not see hotels (%v): %+v", hotels.SheratonID, mpv)
	}
}
