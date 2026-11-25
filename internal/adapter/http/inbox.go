package httpadapter

import (
	"net/http"

	appinbox "github.com/dua-os/dua-os-be/internal/app/inbox"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/go-chi/chi/v5"
)

const maxInboxBody = 256 << 10

func (a API) inboxGate(w http.ResponseWriter, r *http.Request) (*appinbox.Service, bool) {
	if a.Inbox == nil {
		httpx.WriteError(w, errx.New("inbox_unavailable", "the inbox is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxInboxBody)
	return a.Inbox, true
}

func (a API) inboxStatus(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Status(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) inboxList(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.List(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) inboxSync(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Sync(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) inboxGet(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Get(r.Context(), ac, chi.URLParam(r, "messageID"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) inboxEnrich(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Enrich(r.Context(), ac, chi.URLParam(r, "messageID"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) inboxConnectDemo(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.ConnectDemo(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) inboxDisconnect(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	if err := svc.Disconnect(r.Context(), ac); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a API) inboxStartOutlook(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	authURL, err := svc.StartOutlook(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"authUrl": authURL})
}

func (a API) inboxOutlookCallback(w http.ResponseWriter, r *http.Request) {
	if a.Inbox == nil {
		http.Error(w, "inbox unavailable", http.StatusNotImplemented)
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	redirect, err := a.Inbox.CompleteOutlook(r.Context(), code, state)
	if redirect == "" {
		redirect = "/"
	}
	if err != nil {
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (a API) inboxTranslateCompose(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Text       string `json:"text"`
		TargetLang string `json:"targetLang"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.TranslateCompose(r.Context(), ac, body.Text, body.TargetLang)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) inboxSend(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Body    string   `json:"body"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := svc.Send(r.Context(), ac, body.To, body.Subject, body.Body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a API) inboxSaveOutlookApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.inboxGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		Tenant       string `json:"tenant"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.SaveOutlookApp(r.Context(), ac, body.ClientID, body.ClientSecret, body.Tenant)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
