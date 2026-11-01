package httpadapter

import (
	"net/http"

	appcafepos "github.com/dua-os/dua-os-be/internal/app/cafepos"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

func (a API) posGate(w http.ResponseWriter, r *http.Request) (*appcafepos.Service, bool) {
	if a.POS == nil {
		httpx.WriteError(w, errx.New("pos_unavailable", "POS is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxQRBody)
	return a.POS, true
}

func (a API) getCafePOS(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.posGate(w, r)
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

func (a API) recordCafePOS(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.posGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		BusinessDate string `json:"businessDate"`
		ExternalRef  string `json:"externalRef"`
		Kind         string `json:"kind"`
		Tender       string `json:"tender"`
		Lines        int    `json:"lines"`
		Gross        int64  `json:"gross"`
		Discount     int64  `json:"discount"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Record(r.Context(), ac, body.BusinessDate, body.ExternalRef, body.Kind, body.Tender, body.Lines, body.Gross, body.Discount)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) closeCafePOS(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.posGate(w, r)
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
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Close(r.Context(), ac, body.BusinessDate)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
