package httpadapter

import (
	"net/http"

	apphotelpms "github.com/dua-os/dua-os-be/internal/app/hotelpms"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const maxHotelBody = 512 << 10

func (a API) hotelPMSGate(w http.ResponseWriter, r *http.Request) (*apphotelpms.Service, bool) {
	if a.HotelPMS == nil {
		httpx.WriteError(w, errx.New("hotel_pms_unavailable", "hotel PMS is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxHotelBody)
	return a.HotelPMS, true
}

func (a API) getHotelPMS(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.hotelPMSGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.GetDashboard(r.Context(), ac, r.URL.Query().Get("date"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) putHotelOHIP(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.hotelPMSGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled      bool   `json:"enabled"`
		HostURL      string `json:"hostUrl"`
		HotelID      string `json:"hotelId"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		AppKey       string `json:"appKey"`
		SyncMode     string `json:"syncMode"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.SaveOHIPConfig(r.Context(), ac, apphotelpms.SaveOHIPInput{
		Enabled: body.Enabled, HostURL: body.HostURL, HotelID: body.HotelID,
		ClientID: body.ClientID, ClientSecret: body.ClientSecret, AppKey: body.AppKey,
		SyncMode: body.SyncMode,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) syncHotelOHIP(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.hotelPMSGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		BusinessDate string `json:"businessDate"`
	}
	if r.ContentLength > 0 {
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	view, err := svc.SyncOHIP(r.Context(), ac, body.BusinessDate)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) importHotelPMSSnapshot(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.hotelPMSGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		BusinessDate      string                      `json:"businessDate"`
		Source            string                      `json:"source"`
		ExternalBatchRef  string                      `json:"externalBatchRef"`
		Currency          string                      `json:"currency"`
		TotalRooms        int                         `json:"totalRooms"`
		OccupiedRooms     int                         `json:"occupiedRooms"`
		OutOfOrderRooms   int                         `json:"outOfOrderRooms"`
		RoomRevenueMinor  int64                       `json:"roomRevenueMinor"`
		FBRevenueMinor    int64                       `json:"fbRevenueMinor"`
		OtherRevenueMinor int64                       `json:"otherRevenueMinor"`
		ArrivalsCount     int                         `json:"arrivalsCount"`
		DeparturesCount   int                         `json:"departuresCount"`
		InHouseGuests     int                         `json:"inHouseGuests"`
		ReservationsCount int                         `json:"reservationsCount"`
		CancelledCount    int                         `json:"cancelledCount"`
		NoShowCount       int                         `json:"noShowCount"`
		Readiness         []hotel.DepartmentReadiness `json:"readiness"`
		Reservations      []hotel.ReservationRow      `json:"reservations"`
		RawContent        string                      `json:"rawContent"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	var (
		view apphotelpms.DashboardView
		err  error
	)
	if body.RawContent != "" {
		view, err = svc.ImportOPERAFile(r.Context(), ac, body.BusinessDate, body.ExternalBatchRef, body.RawContent)
	} else {
		view, err = svc.ImportSnapshot(r.Context(), ac, apphotelpms.SnapshotInput{
			BusinessDate: body.BusinessDate, Source: body.Source, ExternalBatchRef: body.ExternalBatchRef,
			Currency: body.Currency, TotalRooms: body.TotalRooms, OccupiedRooms: body.OccupiedRooms,
			OutOfOrderRooms: body.OutOfOrderRooms, RoomRevenueMinor: body.RoomRevenueMinor,
			FBRevenueMinor: body.FBRevenueMinor, OtherRevenueMinor: body.OtherRevenueMinor,
			ArrivalsCount: body.ArrivalsCount, DeparturesCount: body.DeparturesCount,
			InHouseGuests: body.InHouseGuests, ReservationsCount: body.ReservationsCount,
			CancelledCount: body.CancelledCount, NoShowCount: body.NoShowCount,
			Readiness: body.Readiness, Reservations: body.Reservations,
		})
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
