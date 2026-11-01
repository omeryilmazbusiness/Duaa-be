package httpadapter

import (
	"net/http"

	appcafeintake "github.com/dua-os/dua-os-be/internal/app/cafeintake"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

func (a API) intakeGate(w http.ResponseWriter, r *http.Request) (*appcafeintake.Service, bool) {
	if a.Intake == nil {
		httpx.WriteError(w, errx.New("intake_unavailable", "daily close is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxQRBody)
	return a.Intake, true
}

func (a API) getCafeIntake(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.intakeGate(w, r)
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

func (a API) listCafeIntakes(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.intakeGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := svc.List(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (a API) putCafeIntake(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.intakeGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		BusinessDate string `json:"businessDate"`
		ShiftName    string `json:"shiftName"`
		CloseNote    string `json:"closeNote"`
		WasteNote    string `json:"wasteNote"`
		IncidentNote string `json:"incidentNote"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Save(r.Context(), ac, appcafeintake.Draft{
		BusinessDate: body.BusinessDate,
		ShiftName:    body.ShiftName,
		CloseNote:    body.CloseNote,
		WasteNote:    body.WasteNote,
		IncidentNote: body.IncidentNote,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
