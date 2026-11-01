package httpadapter

import (
	"net/http"

	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

func (a API) listReports(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := a.Reports.List(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (a API) createReport(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind   string `json:"kind"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Reports.Submit(r.Context(), ac, body.Kind, body.Title, body.Detail)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) actReport(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Action         string `json:"action"`
		AssigneeUserId string `json:"assigneeUserId"`
		Title          string `json:"title"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Reports.Act(r.Context(), ac, chi.URLParam(r, "id"), body.Action, body.AssigneeUserId, body.Title)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
