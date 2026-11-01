package httpadapter

import (
	"net/http"

	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const maxAssistantBody = 64 << 10

// assistantGate resolves the actor and guards against an unwired assistant.
func (a API) assistantGate(w http.ResponseWriter, r *http.Request) (*appassistant.Service, bool) {
	if a.Assistant == nil {
		httpx.WriteError(w, errx.New("ai_unavailable", "the AI assistant is not available", errx.ErrNotImplemented))
		return nil, false
	}
	// Settings and answers can be sensitive; never let a proxy or browser cache them.
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxAssistantBody)
	return a.Assistant, true
}

func (a API) assistantSettings(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.GetSettings(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) saveAssistantSettings(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Provider   string `json:"provider"`
		Model      string `json:"model"`
		APIKey     string `json:"apiKey"`
		DailyLimit int    `json:"dailyLimit"`
		Enabled    bool   `json:"enabled"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.SaveSettings(r.Context(), ac, appassistant.SaveInput{
		Provider: body.Provider, Model: body.Model, APIKey: body.APIKey,
		DailyLimit: body.DailyLimit, Enabled: body.Enabled,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) deleteAssistantSettings(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	if err := svc.RemoveKey(r.Context(), ac); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a API) assistantStatus(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
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

func (a API) assistantChat(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Locale string `json:"locale"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	msgs := make([]assistant.Message, 0, len(body.Messages))
	for _, m := range body.Messages {
		msgs = append(msgs, assistant.Message{Role: assistant.Role(m.Role), Content: m.Content})
	}
	view, err := svc.Chat(r.Context(), ac, appassistant.ChatInput{Messages: msgs, Locale: body.Locale})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) assistantModels(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	models, err := svc.Models(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"models": models})
}
