package httpadapter

import (
	"net/http"

	"github.com/dua-os/dua-os-be/internal/adapter/qrimg"
	appcafeqr "github.com/dua-os/dua-os-be/internal/app/cafeqr"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/go-chi/chi/v5"
)

const maxQRBody = 8 << 10

func (a API) qrGate(w http.ResponseWriter, r *http.Request) (*appcafeqr.Service, bool) {
	if a.QR == nil {
		httpx.WriteError(w, errx.New("qr_unavailable", "QR codes are not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxQRBody)
	return a.QR, true
}

func (a API) listCafeQR(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := svc.ListCodes(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (a API) rotateCafeQR(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Rotate(r.Context(), ac, chi.URLParam(r, "siteID"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) actCafeVoice(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Action         string `json:"action"`
		AssigneeUserId string `json:"assigneeUserId"`
		Note           string `json:"note"`
		Amount         int64  `json:"amount"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Act(r.Context(), ac, chi.URLParam(r, "id"), body.Action, body.AssigneeUserId, body.Note, body.Amount)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) listCafeVoices(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	list, err := svc.ListVoices(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (a API) publicQR(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	view, err := svc.Resolve(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) publicQRImage(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	target, err := svc.ImageTarget(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	raw, err := qrimg.SVG(target)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (a API) issueCafeTable(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		TableNumber string `json:"tableNumber"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.IssueTable(r.Context(), ac, body.TableNumber)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) renameCafeTable(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		TableNumber string `json:"tableNumber"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.RenameTable(r.Context(), ac, chi.URLParam(r, "siteID"), body.TableNumber)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) submitPublicQR(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.qrGate(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind         string `json:"kind"`
		Rating       int    `json:"rating"`
		Comment      string `json:"comment"`
		ContactName  string `json:"contactName"`
		ContactPhone string `json:"contactPhone"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Submit(r.Context(), chi.URLParam(r, "token"), appcafeqr.SubmitInput{
		Kind: body.Kind, Rating: body.Rating, Comment: body.Comment, ContactName: body.ContactName, ContactPhone: body.ContactPhone,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}
