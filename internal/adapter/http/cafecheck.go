package httpadapter

import (
	"net/http"

	appcafecheck "github.com/dua-os/dua-os-be/internal/app/cafecheck"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

func (a API) checkGate(w http.ResponseWriter, r *http.Request) (*appcafecheck.Service, bool) {
	if a.Check == nil {
		httpx.WriteError(w, errx.New("checklist_unavailable", "opening list is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxQRBody)
	return a.Check, true
}

func (a API) getCafeChecklist(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.checkGate(w, r)
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

func (a API) putCafeChecklist(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.checkGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Items []struct {
			ID     string `json:"id"`
			Kind   string `json:"kind"`
			Title  string `json:"title"`
			Active *bool  `json:"active"`
		} `json:"items"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	in := make([]appcafecheck.DraftItem, 0, len(body.Items))
	for _, row := range body.Items {
		in = append(in, appcafecheck.DraftItem{ID: row.ID, Kind: row.Kind, Title: row.Title, Active: row.Active})
	}
	view, err := svc.SaveTemplate(r.Context(), ac, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) toggleCafeChecklist(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.checkGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		BusinessDate string `json:"businessDate"`
		ItemID       string `json:"itemId"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Toggle(r.Context(), ac, body.BusinessDate, body.ItemID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
