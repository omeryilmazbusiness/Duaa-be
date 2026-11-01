package httpadapter

import (
	"net/http"

	appcafehandover "github.com/dua-os/dua-os-be/internal/app/cafehandover"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/go-chi/chi/v5"
)

func (a API) handoverGate(w http.ResponseWriter, r *http.Request) (*appcafehandover.Service, bool) {
	if a.Handover == nil {
		httpx.WriteError(w, errx.New("handover_unavailable", "handover is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxQRBody)
	return a.Handover, true
}

func (a API) getCafeHandover(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.handoverGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Get(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) listCafeHandovers(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.handoverGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := svc.List(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (a API) openCafeHandover(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.handoverGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		BusinessDate string `json:"businessDate"`
		FromShift    string `json:"fromShift"`
		ToShift      string `json:"toShift"`
		Note         string `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Open(r.Context(), ac, appcafehandover.Draft{
		BusinessDate: body.BusinessDate,
		FromShift:    body.FromShift,
		ToShift:      body.ToShift,
		Note:         body.Note,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) acceptCafeHandover(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.handoverGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Accept(r.Context(), ac, chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
