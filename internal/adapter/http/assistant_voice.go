package httpadapter

import (
	"io"
	"net/http"
	"strings"

	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

func (a API) assistantTranslate(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Text       string `json:"text"`
		SourceLang string `json:"sourceLang"`
		TargetLang string `json:"targetLang"`
		Mode       string `json:"mode"`
		Context    string `json:"context"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Translate(r.Context(), ac, appassistant.TranslateInput{
		Text: body.Text, SourceLang: body.SourceLang, TargetLang: body.TargetLang,
		Mode: body.Mode, Context: body.Context,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) assistantMeetingsList(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	items, err := svc.ListMeetings(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"meetings": items})
}

func (a API) assistantMeetingsSummarize(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Title        string   `json:"title"`
		Participants []string `json:"participants"`
		Transcript   string   `json:"transcript"`
		SourceLocale string   `json:"sourceLocale"`
		TargetLocale string   `json:"targetLocale"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.SummarizeMeeting(r.Context(), ac, appassistant.MeetingInput{
		Title: body.Title, Participants: body.Participants, Transcript: body.Transcript,
		SourceLocale: body.SourceLocale, TargetLocale: body.TargetLocale,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) assistantAudioTranscribe(w http.ResponseWriter, r *http.Request) {
	if a.Assistant == nil {
		httpx.WriteError(w, errx.New("ai_unavailable", "the AI assistant is not available", errx.ErrNotImplemented))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, assistant.MaxAudioBytes+(64<<10))
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(assistant.MaxAudioBytes + (64 << 10)); err != nil {
		httpx.WriteError(w, errx.Invalid("invalid_audio_upload", "audio form upload is invalid"))
		return
	}
	file, hdr, err := r.FormFile("audio")
	if err != nil {
		httpx.WriteError(w, errx.Invalid("audio_required", "audio file is required"))
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, assistant.MaxAudioBytes+1))
	if err != nil {
		httpx.WriteError(w, errx.Invalid("audio_unreadable", "could not read audio clip"))
		return
	}
	mime := hdr.Header.Get("Content-Type")
	if mime == "" {
		mime = "audio/webm"
	}
	view, err := a.Assistant.TranscribeAudio(r.Context(), ac, appassistant.AudioInput{
		Audio: raw, MimeType: mime, FileName: hdr.Filename, LanguageHint: r.FormValue("language"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) assistantAudioSpeak(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.assistantGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	audio, contentType, err := svc.SpeakAudio(r.Context(), ac, appassistant.SpeakInput{
		Text: body.Text, Language: body.Language,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "audio/mpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
}
