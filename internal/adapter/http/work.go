package httpadapter

import (
	"net/http"
	"time"

	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/go-chi/chi/v5"
)

func (a API) listTasks(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := a.Tasks.List(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (a API) createTask(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Title          string `json:"title"`
		Description    string `json:"description"`
		AssigneeUserId string `json:"assigneeUserId"`
		StartAt        string `json:"scheduledStartAt"`
		EndAt          string `json:"scheduledEndAt"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	start, err := time.Parse(time.RFC3339, body.StartAt)
	if err != nil {
		httpx.WriteError(w, errx.Invalid("invalid_start", "scheduledStartAt must be RFC3339"))
		return
	}
	end, err := time.Parse(time.RFC3339, body.EndAt)
	if err != nil {
		httpx.WriteError(w, errx.Invalid("invalid_end", "scheduledEndAt must be RFC3339"))
		return
	}
	view, err := a.Tasks.Create(r.Context(), ac, body.Title, body.Description, body.AssigneeUserId, start, end)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) listRoutines(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := a.Tasks.ListRoutines(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (a API) createRoutine(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Title          string `json:"title"`
		Description    string `json:"description"`
		AssigneeUserId string `json:"assigneeUserId"`
		StartTime      string `json:"startTime"`
		EndTime        string `json:"endTime"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	rOut, err := a.Tasks.CreateRoutine(r.Context(), ac, body.Title, body.Description, body.AssigneeUserId, body.StartTime, body.EndTime)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rOut)
}

func (a API) portalTasks(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := a.Tasks.PortalTasks(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (a API) completeTask(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := a.Tasks.Complete(r.Context(), ac, chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) completePortalTask(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := a.Tasks.CompletePortal(r.Context(), ac, chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
