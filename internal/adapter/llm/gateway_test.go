package llm_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dua-os/dua-os-be/internal/adapter/llm"
	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

func request(p assistant.Provider, model string) appassistant.CompletionRequest {
	return appassistant.CompletionRequest{
		Provider: p, Model: model, APIKey: "secret-key-1234567890", System: "be helpful", MaxOutputTokens: 100,
		Messages: []assistant.Message{
			{Role: assistant.RoleUser, Content: "hello"},
			{Role: assistant.RoleAssistant, Content: "hi"},
			{Role: assistant.RoleUser, Content: "summary?"},
		},
	}
}

func gateway(base string) *llm.Gateway {
	return llm.NewGateway(base, base)
}

func TestOpenAIRequestAndReply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret-key-1234567890" {
			t.Errorf("bad request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct{ Role, Content string }
			Max      int `json:"max_completion_tokens"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if body.Model != "gpt-4o-mini" || body.Max != 100 || len(body.Messages) != 4 || body.Messages[0].Role != "system" || body.Messages[3].Content != "summary?" {
			t.Errorf("unexpected payload: %s", raw)
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"  All good  "}}]}`)
	}))
	defer srv.Close()
	res, err := gateway(srv.URL).Complete(t.Context(), request(assistant.ProviderOpenAI, "gpt-4o-mini"))
	if err != nil || res.Text != "All good" {
		t.Fatalf("res = %q, err = %v", res.Text, err)
	}
}

func TestGeminiRequestAndReply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-2.0-flash:generateContent" || r.Header.Get("x-goog-api-key") != "secret-key-1234567890" {
			t.Errorf("bad request %s key=%q", r.URL.String(), r.Header.Get("x-goog-api-key"))
		}
		if strings.Contains(r.URL.RawQuery, "secret") {
			t.Error("key must not travel in the URL")
		}
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			SystemInstruction struct{ Parts []struct{ Text string } } `json:"systemInstruction"`
			Contents          []struct{ Role string }                 `json:"contents"`
		}
		_ = json.Unmarshal(raw, &body)
		if body.SystemInstruction.Parts[0].Text != "be helpful" || len(body.Contents) != 3 || body.Contents[1].Role != "model" {
			t.Errorf("unexpected payload: %s", raw)
		}
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"Hello "},{"text":"there"}]}}]}`)
	}))
	defer srv.Close()
	res, err := gateway(srv.URL).Complete(t.Context(), request(assistant.ProviderGemini, "gemini-2.0-flash"))
	if err != nil || res.Text != "Hello there" {
		t.Fatalf("res = %q, err = %v", res.Text, err)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
		code   string
	}{
		{"unauthorized", 401, `{"error":{"message":"Incorrect API key provided: secret-key-1234567890"}}`, errx.ErrInvalid, "ai_key_rejected"},
		{"gemini bad key", 400, `{"error":{"message":"API key not valid. Please pass a valid API key."}}`, errx.ErrInvalid, "ai_key_rejected"},
		{"gemini model 400", 400, `{"error":{"message":"models/gemini-2.5-flash is not found for API version v1beta"}}`, errx.ErrInvalid, "ai_model_not_found"},
		{"region", 403, `{"error":{"message":"User location is not supported for the API use."}}`, errx.ErrInvalid, "ai_region_blocked"},
		{"model", 404, `{}`, errx.ErrInvalid, "ai_model_not_found"},
		{"quota", 429, `{}`, errx.ErrRateLimited, "ai_provider_rate_limited"},
		{"server", 503, `{}`, errx.ErrUpstream, "ai_provider_unavailable"},
		{"bad request", 400, `{"error":{"message":"unsupported parameter secret-key-1234567890"}}`, errx.ErrInvalid, "ai_request_rejected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			_, err := gateway(srv.URL).Complete(t.Context(), request(assistant.ProviderOpenAI, "gpt-4o-mini"))
			var xe *errx.Error
			if !errors.Is(err, tc.want) || !errors.As(err, &xe) || xe.Code != tc.code {
				t.Fatalf("err = %v, want %v/%s", err, tc.want, tc.code)
			}
			if strings.Contains(err.Error(), "secret-key-1234567890") {
				t.Fatalf("error leaks the key: %v", err)
			}
		})
	}
}

func TestGeminiFallsBackToV1WhenV1BetaLacksModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1beta/models/gemini-2.5-flash:generateContent" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"message":"not found"}}`)
			return
		}
		if r.URL.Path != "/v1/models/gemini-2.5-flash:generateContent" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	}))
	defer srv.Close()
	res, err := gateway(srv.URL).Complete(t.Context(), request(assistant.ProviderGemini, "gemini-2.5-flash"))
	if err != nil || res.Text != "ok" {
		t.Fatalf("res = %q, err = %v", res.Text, err)
	}
}

func TestUnreachableProvider(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	_, err := gateway(url).Complete(t.Context(), request(assistant.ProviderOpenAI, "gpt-4o-mini"))
	if !errors.Is(err, errx.ErrUpstream) {
		t.Fatalf("err = %v, want upstream", err)
	}
}

func TestGeminiListModelsPaginatesAndFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models" || r.Header.Get("x-goog-api-key") != "secret-key-1234567890" || strings.Contains(r.URL.RawQuery, "secret") {
			t.Errorf("bad request %s", r.URL.String())
		}
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = io.WriteString(w, `{"models":[
				{"name":"models/gemini-2.5-flash","supportedGenerationMethods":["generateContent"]},
				{"name":"models/gemini-embedding-001","supportedGenerationMethods":["embedContent"]},
				{"name":"models/text-bison","supportedGenerationMethods":["generateContent"]}],
				"nextPageToken":"p2"}`)
			return
		}
		_, _ = io.WriteString(w, `{"models":[{"name":"models/gemini-3-pro-preview","supportedGenerationMethods":["generateContent","countTokens"]}]}`)
	}))
	defer srv.Close()
	ids, err := gateway(srv.URL).ListModels(t.Context(), assistant.ProviderGemini, "secret-key-1234567890")
	if err != nil || len(ids) != 2 || ids[0] != "gemini-2.5-flash" || ids[1] != "gemini-3-pro-preview" {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
}

func TestOpenAIListModelsFiltersChatIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer secret-key-1234567890" {
			t.Errorf("bad request %s", r.URL.String())
		}
		_, _ = io.WriteString(w, `{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4o-mini-instruct"},{"id":"whisper-1"}]}`)
	}))
	defer srv.Close()
	ids, err := gateway(srv.URL).ListModels(t.Context(), assistant.ProviderOpenAI, "secret-key-1234567890")
	if err != nil || len(ids) != 1 || ids[0] != "gpt-4o-mini" {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
}

func TestGeminiFlashDisablesThinking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), `"thinkingBudget":0`) {
			t.Errorf("2.5 flash must disable thinking: %s", raw)
		}
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	}))
	defer srv.Close()
	res, err := gateway(srv.URL).Complete(t.Context(), request(assistant.ProviderGemini, "gemini-2.5-flash"))
	if err != nil || res.Text != "ok" {
		t.Fatalf("res = %q, err = %v", res.Text, err)
	}
}

func TestGemini31FlashLiteOmitsThinking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "thinkingConfig") || strings.Contains(string(raw), "thinkingBudget") {
			t.Errorf("3.1 flash-lite must not send thinkingConfig: %s", raw)
		}
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	}))
	defer srv.Close()
	res, err := gateway(srv.URL).Complete(t.Context(), request(assistant.ProviderGemini, "gemini-3.1-flash-lite"))
	if err != nil || res.Text != "ok" {
		t.Fatalf("res = %q, err = %v", res.Text, err)
	}
}
