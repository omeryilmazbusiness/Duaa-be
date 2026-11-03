package httpadapter

import (
	"net/http"

	appcafestock "github.com/dua-os/dua-os-be/internal/app/cafestock"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

func (a API) stockGate(w http.ResponseWriter, r *http.Request) (*appcafestock.Service, bool) {
	if a.Stock == nil {
		httpx.WriteError(w, errx.New("stock_unavailable", "stock is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxQRBody)
	return a.Stock, true
}

func (a API) getCafeStock(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.stockGate(w, r)
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

func (a API) putCafeStockCatalog(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.stockGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Items []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Station string `json:"station"`
			Unit    string `json:"unit"`
			Par     int64  `json:"par"`
			Active  *bool  `json:"active"`
		} `json:"items"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	in := make([]appcafestock.ItemDraft, 0, len(body.Items))
	for _, row := range body.Items {
		active := true
		if row.Active != nil {
			active = *row.Active
		}
		in = append(in, appcafestock.ItemDraft{
			ID: row.ID, Name: row.Name, Station: row.Station, Unit: row.Unit, Par: row.Par, Active: active,
		})
	}
	view, err := svc.SaveCatalog(r.Context(), ac, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) recordCafeStockAdjustment(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.stockGate(w, r)
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
		Reason       string `json:"reason"`
		Note         string `json:"note"`
		OnHand       *int64 `json:"onHand"`
		Delta        *int64 `json:"delta"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Adjust(r.Context(), ac, body.BusinessDate, body.ItemID, body.Reason, body.Note, body.OnHand, body.Delta)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}
