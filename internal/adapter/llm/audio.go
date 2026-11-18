package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

var _ appassistant.AudioGateway = (*Gateway)(nil)

func (g *Gateway) Transcribe(ctx context.Context, req appassistant.AudioTranscribeRequest) (appassistant.AudioTranscribeResult, error) {
	switch req.Provider {
	case assistant.ProviderOpenAI:
		return g.openAITranscribe(ctx, req)
	case assistant.ProviderGemini:
		return g.geminiTranscribe(ctx, req)
	default:
		return appassistant.AudioTranscribeResult{}, errx.Invalid("ai_provider_invalid", "unsupported provider")
	}
}

func (g *Gateway) Speak(ctx context.Context, req appassistant.AudioSpeakRequest) ([]byte, string, error) {
	if req.Provider != assistant.ProviderOpenAI {
		return nil, "", errx.New("tts_browser_fallback", "use browser speech synthesis", errx.ErrNotImplemented)
	}
	body := map[string]any{
		"model":           "tts-1",
		"input":           req.Text,
		"voice":           "nova",
		"response_format": "mp3",
	}
	raw, err := g.post(ctx, strings.TrimRight(g.OpenAIBase, "/")+"/v1/audio/speech",
		map[string]string{"Authorization": "Bearer " + req.APIKey}, body, req.APIKey)
	if err != nil {
		return nil, "", err
	}
	return raw, "audio/mpeg", nil
}

func (g *Gateway) openAITranscribe(ctx context.Context, req appassistant.AudioTranscribeRequest) (appassistant.AudioTranscribeResult, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("model", "whisper-1")
	if req.LanguageHint != "" && req.LanguageHint != "auto" {
		_ = mw.WriteField("language", req.LanguageHint)
	}
	_ = mw.WriteField("response_format", "json")
	name := req.FileName
	if name == "" {
		name = "voice.webm"
	}
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		return appassistant.AudioTranscribeResult{}, err
	}
	if _, err := fw.Write(req.Audio); err != nil {
		return appassistant.AudioTranscribeResult{}, err
	}
	_ = mw.Close()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(g.OpenAIBase, "/")+"/v1/audio/transcriptions", &buf)
	if err != nil {
		return appassistant.AudioTranscribeResult{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return appassistant.AudioTranscribeResult{}, transportError(ctx, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return appassistant.AudioTranscribeResult{}, transportError(ctx, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return appassistant.AudioTranscribeResult{}, statusError(resp.StatusCode, data, req.APIKey)
	}
	var out struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if json.Unmarshal(data, &out) != nil {
		return appassistant.AudioTranscribeResult{}, errx.Upstream("ai_provider_bad_response", "could not parse transcription")
	}
	lang := out.Language
	if lang == "" {
		lang = req.LanguageHint
	}
	return appassistant.AudioTranscribeResult{Text: strings.TrimSpace(out.Text), DetectedLanguage: lang}, nil
}

func (g *Gateway) geminiTranscribe(ctx context.Context, req appassistant.AudioTranscribeRequest) (appassistant.AudioTranscribeResult, error) {
	mime := strings.Split(strings.TrimSpace(req.MimeType), ";")[0]
	if mime == "" {
		mime = "audio/webm"
	}
	model := req.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}
	prompt := "Transcribe this speech accurately verbatim in its original spoken language. Return ONLY JSON: {\"text\":\"...\",\"language\":\"en|ar|tr\"}."
	if req.LanguageHint != "" && req.LanguageHint != "auto" {
		prompt = "Transcribe this speech accurately verbatim (expected language: " + req.LanguageHint + "). Return ONLY JSON: {\"text\":\"...\",\"language\":\"" + req.LanguageHint + "\"}."
	}
	payload := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{
				{"text": prompt},
				{"inlineData": map[string]any{
					"mimeType": mime,
					"data":     base64.StdEncoding.EncodeToString(req.Audio),
				}},
			},
		}},
	}
	endpoint := strings.TrimRight(g.GeminiBase, "/") + "/v1beta/models/" + url.PathEscape(model) + ":generateContent"
	data, err := g.post(ctx, endpoint, map[string]string{"x-goog-api-key": req.APIKey}, payload, req.APIKey)
	if err != nil {
		return appassistant.AudioTranscribeResult{}, err
	}
	res, err := parseGemini(data)
	if err != nil {
		return appassistant.AudioTranscribeResult{}, err
	}
	var parsed struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if obj := extractJSONObject(res.Text); obj != "" && json.Unmarshal([]byte(obj), &parsed) == nil && strings.TrimSpace(parsed.Text) != "" {
		return appassistant.AudioTranscribeResult{Text: strings.TrimSpace(parsed.Text), DetectedLanguage: parsed.Language}, nil
	}
	return appassistant.AudioTranscribeResult{Text: strings.TrimSpace(res.Text), DetectedLanguage: req.LanguageHint}, nil
}
