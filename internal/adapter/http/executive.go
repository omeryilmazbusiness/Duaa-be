package httpadapter

import (
	"net/http"

	appdecision "github.com/dua-os/dua-os-be/internal/app/decision"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
)

func (a API) brief(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	if a.Decisions != nil {
		if err := a.Decisions.Reconcile(r.Context(), ac); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	b, err := a.Brief.ForGM(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b)
}

func (a API) actDecision(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		SourceKey      string `json:"sourceKey"`
		Fingerprint    string `json:"fingerprint"`
		Kind           string `json:"kind"`
		TitleKey       string `json:"titleKey"`
		Detail         string `json:"detail"`
		Severity       string `json:"severity"`
		TargetTab      string `json:"targetTab"`
		Action         string `json:"action"`
		AssigneeUserId string `json:"assigneeUserId"`
		Title          string `json:"title"`
		Description    string `json:"description"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Decisions.Act(r.Context(), ac, appdecision.Command{
		SourceKey:      body.SourceKey,
		Fingerprint:    body.Fingerprint,
		Kind:           body.Kind,
		TitleKey:       body.TitleKey,
		Detail:         body.Detail,
		Severity:       body.Severity,
		TargetTab:      body.TargetTab,
		Action:         body.Action,
		AssigneeUserID: body.AssigneeUserId,
		Title:          body.Title,
		Description:    body.Description,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
