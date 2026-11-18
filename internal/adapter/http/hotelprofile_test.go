package httpadapter_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/bootstrap"
)

func TestHotelProfileEditAndWorkspaceIsolation(t *testing.T) {
	h := newHarness(t)
	hotels, err := bootstrap.SeedPortfolioHotels(t.Context(), h.store, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	login := post(h.handler, "/v1/auth/login", map[string]string{"email": h.seed.GMEmail, "password": h.seed.GMPassword}, "")
	if login.Code != 200 {
		t.Fatalf("login %d %s", login.Code, login.Body.String())
	}
	var cafe wrapLogin
	decode(t, login, &cafe)

	path := "/v1/workspaces/" + hotels.AlShohadaID.String() + "/hotel/profile"
	body := map[string]any{"displayName": "Al Shohada Grand", "totalRooms": 140, "currency": "IQD", "starRating": 4, "city": "Karbala"}
	if put(h.handler, path, body, cafe.Token).Code != 403 {
		t.Fatal("a cafe-scoped token must not edit another workspace's hotel profile")
	}

	switched := post(h.handler, "/v1/me/workspaces/"+hotels.AlShohadaID.String()+"/select", nil, cafe.Token)
	if switched.Code != 200 {
		t.Fatalf("select %d %s", switched.Code, switched.Body.String())
	}
	var hotel wrapLogin
	decode(t, switched, &hotel)

	seed := get(h.handler, path, hotel.Token)
	if seed.Code != 200 {
		t.Fatalf("get profile %d %s", seed.Code, seed.Body.String())
	}
	var seedView map[string]any
	decode(t, seed, &seedView)
	if seedView["persisted"] != false || seedView["canWrite"] != true {
		t.Fatalf("seed view %+v", seedView)
	}

	saved := put(h.handler, path, body, hotel.Token)
	if saved.Code != 200 {
		t.Fatalf("put profile %d %s", saved.Code, saved.Body.String())
	}
	var view map[string]any
	decode(t, saved, &view)
	if view["displayName"] != "Al Shohada Grand" || view["currency"] != "IQD" || view["persisted"] != true {
		t.Fatalf("saved %+v", view)
	}

	var list struct {
		Workspaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"workspaces"`
	}
	decode(t, get(h.handler, "/v1/me/workspaces", hotel.Token), &list)
	renamed := false
	for _, w := range list.Workspaces {
		if w.ID == hotels.AlShohadaID.String() && w.Name == "Al Shohada Grand" {
			renamed = true
		}
	}
	if !renamed {
		t.Fatalf("workspace list must show the new hotel name: %+v", list.Workspaces)
	}

	bad := put(h.handler, path, map[string]any{"displayName": "X", "currency": "dollars"}, hotel.Token)
	if bad.Code != 422 {
		t.Fatalf("invalid currency must be 422, got %d %s", bad.Code, bad.Body.String())
	}
}
