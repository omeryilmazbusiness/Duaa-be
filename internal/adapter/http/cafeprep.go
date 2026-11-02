package httpadapter

import (
	"net/http"

	appcafeprep "github.com/dua-os/dua-os-be/internal/app/cafeprep"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/go-chi/chi/v5"
)

func (a API) prepGate(w http.ResponseWriter, r *http.Request) (*appcafeprep.Service, bool) {
	if a.Prep == nil {
		httpx.WriteError(w, errx.New("prep_unavailable", "prep times are not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxQRBody)
	return a.Prep, true
}

func (a API) getCafePrep(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.prepGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Get(r.Context(), ac, r.URL.Query().Get("date"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) recordCafePrep(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.prepGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		BusinessDate string `json:"businessDate"`
		Station      string `json:"station"`
		TicketRef    string `json:"ticketRef"`
		TableLabel   string `json:"tableLabel"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Record(r.Context(), ac, body.BusinessDate, body.Station, body.TicketRef, body.TableLabel)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) readyCafePrep(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.prepGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.MarkReady(r.Context(), ac, chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) serveCafePrep(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.prepGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.MarkServed(r.Context(), ac, chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
