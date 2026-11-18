package httpadapter

import (
	"net/http"

	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

func (a API) executivePortfolio(w http.ResponseWriter, r *http.Request) {
	if a.Portfolio == nil {
		httpx.WriteError(w, errx.New("portfolio_unavailable", "portfolio is not available", errx.ErrNotImplemented))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	p, err := Principal(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Portfolio.Get(r.Context(), p, r.URL.Query().Get("date"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
