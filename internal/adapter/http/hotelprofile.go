package httpadapter

import (
	"net/http"

	apphotelpms "github.com/dua-os/dua-os-be/internal/app/hotelpms"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
)

func (a API) getHotelProfile(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.hotelPMSGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.GetProfile(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) putHotelProfile(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.hotelPMSGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		DisplayName string `json:"displayName"`
		TotalRooms  int    `json:"totalRooms"`
		Currency    string `json:"currency"`
		Timezone    string `json:"timezone"`
		StarRating  int    `json:"starRating"`
		City        string `json:"city"`
		Address     string `json:"address"`
		Phone       string `json:"phone"`
		GMName      string `json:"gmName"`
		Notes       string `json:"notes"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.SaveProfile(r.Context(), ac, apphotelpms.SaveProfileInput{
		DisplayName: body.DisplayName, TotalRooms: body.TotalRooms, Currency: body.Currency,
		Timezone: body.Timezone, StarRating: body.StarRating, City: body.City,
		Address: body.Address, Phone: body.Phone, GMName: body.GMName, Notes: body.Notes,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
