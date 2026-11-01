package httpadapter

import (
	"net/http"

	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

func (a API) dashboard(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := a.Dashboard.ForGM(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) closeCash(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		CountedAmount  int64  `json:"countedAmount"`
		ExpectedAmount *int64 `json:"expectedAmount"`
		Note           string `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Cash.Close(r.Context(), ac, body.CountedAmount, body.ExpectedAmount, body.Note)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) listApprovals(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := a.Approvals.List(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (a API) createApproval(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Title    string `json:"title"`
		Supplier string `json:"supplier"`
		Amount   int64  `json:"amount"`
		Evidence string `json:"evidence"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Approvals.Submit(r.Context(), ac, body.Title, body.Supplier, body.Evidence, body.Amount)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) actApproval(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Action string `json:"action"`
		Amount *int64 `json:"amount"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Approvals.Act(r.Context(), ac, chi.URLParam(r, "id"), body.Action, body.Amount)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
