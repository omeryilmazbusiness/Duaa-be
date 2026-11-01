package httpadapter

import (
	"net/http"

	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

func (a API) listStaff(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := a.Staff.ListPersonnel(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (a API) createStaff(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		FirstName       string `json:"firstName"`
		LastName        string `json:"lastName"`
		StaffDepartment string `json:"staffDepartment"`
		EmployeeNumber  string `json:"employeeNumber"`
		Email           string `json:"email"`
		Password        string `json:"password"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Staff.CreatePersonnel(r.Context(), ac, body.FirstName, body.LastName, body.StaffDepartment, body.EmployeeNumber, body.Email, body.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) deactivateStaff(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	if err := a.Staff.Deactivate(r.Context(), ac, chi.URLParam(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a API) listDeptManagers(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := a.Staff.ListDepartmentManagers(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (a API) createDeptManager(w http.ResponseWriter, r *http.Request) {
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		FirstName       string `json:"firstName"`
		LastName        string `json:"lastName"`
		Email           string `json:"email"`
		Password        string `json:"password"`
		StaffDepartment string `json:"staffDepartment"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := a.Staff.CreateDepartmentManager(r.Context(), ac, body.FirstName, body.LastName, body.Email, body.Password, body.StaffDepartment)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}
