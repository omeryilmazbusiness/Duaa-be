package httpadapter

import (
	"net/http"

	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

func (a API) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	res, err := a.Auth.LoginEmail(r.Context(), body.Email, body.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (a API) loginSelect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		WorkspaceID string `json:"workspaceId"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	res, err := a.Auth.SelectWorkspace(r.Context(), body.Email, body.Password, body.WorkspaceID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (a API) staffLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkspaceSlug  string `json:"workspaceSlug"`
		EmployeeNumber string `json:"employeeNumber"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	res, err := a.Auth.LoginEmployee(r.Context(), body.WorkspaceSlug, body.EmployeeNumber)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (a API) me(w http.ResponseWriter, r *http.Request) {
	p, err := Principal(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	session, err := a.Auth.SessionFromPrincipal(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, session)
}

func (a API) switchWorkspace(w http.ResponseWriter, r *http.Request) {
	p, err := Principal(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	res, err := a.Auth.SwitchWorkspace(r.Context(), p, chi.URLParam(r, "workspaceID"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
