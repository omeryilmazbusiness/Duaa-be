package httpadapter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const testAIKey = "sk-test-0123456789abcdefWXYZ"

// fakeLLM stands in for the provider gateway and records what it was sent.
type fakeLLM struct {
	mu      sync.Mutex
	calls   []appassistant.CompletionRequest
	err     error
	listErr error
	reply   string
	live    []string
	missing map[string]bool
	hang    bool
}

func (f *fakeLLM) Complete(ctx context.Context, req appassistant.CompletionRequest) (appassistant.CompletionResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	hang := f.hang
	f.mu.Unlock()
	if hang {
		select {
		case <-ctx.Done():
			return appassistant.CompletionResult{}, errx.Upstream("ai_provider_timeout", "the AI provider took too long to answer; try again")
		case <-time.After(30 * time.Second):
			return appassistant.CompletionResult{}, errx.Upstream("ai_provider_timeout", "the AI provider took too long to answer; try again")
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.missing[req.Model] {
		return appassistant.CompletionResult{}, errx.Invalid("ai_model_not_found", "the model was not found for this provider and key")
	}
	if f.err != nil {
		return appassistant.CompletionResult{}, f.err
	}
	reply := f.reply
	if reply == "" {
		reply = "ok"
	}
	return appassistant.CompletionResult{Text: reply}, nil
}

func (f *fakeLLM) ListModels(_ context.Context, _ assistant.Provider, _ string) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.live, nil
}

func (f *fakeLLM) last() appassistant.CompletionRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[len(f.calls)-1]
}

func (f *fakeLLM) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func put(h http.Handler, path string, body any, token string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func del(h http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func question(text string) map[string]any {
	return map[string]any{
		"locale":   "tr",
		"messages": []map[string]string{{"role": "user", "content": text}},
	}
}

func gmToken(t *testing.T, h harness) string {
	t.Helper()
	rec := post(h.handler, "/v1/auth/login", map[string]string{"email": h.seed.GMEmail, "password": h.seed.GMPassword}, "")
	var s wrapLogin
	decode(t, rec, &s)
	return s.Token
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	decode(t, rec, &body)
	return body.Error.Code
}

func TestAssistantSetupChatAndDailyLimit(t *testing.T) {
	h := newHarness(t)
	base := "/v1/workspaces/" + h.seed.CafeID.String() + "/assistant"
	token := gmToken(t, h)

	var fresh struct {
		Provider, Model string
		Providers       []struct {
			ID     string
			Models []struct {
				ID   string
				Free bool
			}
		}
	}
	decode(t, get(h.handler, base+"/settings", token), &fresh)
	if fresh.Provider != "gemini" || fresh.Model != "gemini-3.1-flash-lite" || fresh.Providers[0].ID != "gemini" {
		t.Fatalf("fresh workspace must default to free Gemini: %+v", fresh)
	}
	if n := len(fresh.Providers[0].Models); n < 5 || !fresh.Providers[0].Models[0].Free {
		t.Fatalf("gemini must list its models with the free tier marked: %+v", fresh.Providers[0].Models)
	}

	var st struct {
		Available, Configured bool
		Usage                 struct{ Used, Limit int }
	}
	rec := get(h.handler, base+"/status", token)
	decode(t, rec, &st)
	if rec.Code != 200 || st.Available || st.Configured {
		t.Fatalf("fresh workspace must be unconfigured: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(h.handler, base+"/chat", question("hello"), token); rec.Code != 409 || errCode(t, rec) != "ai_not_configured" {
		t.Fatalf("chat before setup: %d %s", rec.Code, rec.Body.String())
	}

	// First setup requires a key.
	if rec := put(h.handler, base+"/settings", map[string]any{"provider": "openai", "dailyLimit": 2, "enabled": true}, token); rec.Code != 422 {
		t.Fatalf("setup without key: %d", rec.Code)
	}
	save := put(h.handler, base+"/settings", map[string]any{
		"provider": "openai", "model": "gpt-4o-mini", "apiKey": testAIKey, "dailyLimit": 2, "enabled": true,
	}, token)
	if save.Code != 200 {
		t.Fatalf("save: %d %s", save.Code, save.Body.String())
	}
	if strings.Contains(save.Body.String(), testAIKey) || strings.Contains(save.Body.String(), "sk-test") {
		t.Fatalf("response must never echo the key: %s", save.Body.String())
	}
	if h.llm.count() != 1 || h.llm.last().APIKey != testAIKey {
		t.Fatal("a new key must be verified against the provider once")
	}
	got := get(h.handler, base+"/settings", token)
	if strings.Contains(got.Body.String(), testAIKey) || !strings.Contains(got.Body.String(), `"keyHint":"WXYZ"`) {
		t.Fatalf("settings must expose only the hint: %s", got.Body.String())
	}
	if got.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("settings must not be cacheable")
	}

	// The key is sealed at rest and bound to the workspace.
	stored, err := h.store.GetAssistantSettings(t.Context(), h.seed.CafeID)
	if err != nil || strings.Contains(stored.EncryptedKey, "sk-test") {
		t.Fatalf("stored key must be ciphertext: %v", err)
	}
	if plain, err := h.vault.Open(stored.EncryptedKey, h.seed.CafeID.String()); err != nil || plain != testAIKey {
		t.Fatalf("stored key must decrypt for its workspace: %v", err)
	}

	// Updating limit without re-sending the key keeps the stored key.
	if rec := put(h.handler, base+"/settings", map[string]any{"provider": "openai", "model": "gpt-4o-mini", "dailyLimit": 2, "enabled": true}, token); rec.Code != 200 {
		t.Fatalf("keep key: %d %s", rec.Code, rec.Body.String())
	}
	// Changing provider requires a new key.
	if rec := put(h.handler, base+"/settings", map[string]any{"provider": "gemini", "dailyLimit": 2, "enabled": true}, token); rec.Code != 422 {
		t.Fatalf("provider switch without key: %d", rec.Code)
	}

	h.llm.reply = "Bugun 3 gorev acik."
	chat := post(h.handler, base+"/chat", question("Bugunu ozetle"), token)
	if chat.Code != 200 {
		t.Fatalf("chat: %d %s", chat.Code, chat.Body.String())
	}
	var out struct {
		Reply string
		Usage struct{ Used, Limit, Remaining int }
	}
	decode(t, chat, &out)
	if out.Reply != "Bugun 3 gorev acik." || out.Usage.Used != 1 || out.Usage.Remaining != 1 {
		t.Fatalf("unexpected chat result: %+v", out)
	}
	call := h.llm.last()
	for _, want := range []string{"PROTOCOLS", "Duaa Cafe", "<workspace_data>", "Pulse:"} {
		if !strings.Contains(call.System, want) {
			t.Fatalf("system prompt missing %q", want)
		}
	}
	if len(call.Messages) != 1 || call.Messages[0].Content != "Bugunu ozetle" {
		t.Fatalf("messages = %+v", call.Messages)
	}

	// A provider failure must not consume quota.
	h.llm.err = errx.Upstream("ai_provider_unavailable", "down")
	if rec := post(h.handler, base+"/chat", question("again"), token); rec.Code != 502 {
		t.Fatalf("provider failure: %d %s", rec.Code, rec.Body.String())
	}
	h.llm.err = nil
	decode(t, get(h.handler, base+"/status", token), &st)
	if st.Usage.Used != 1 {
		t.Fatalf("failed call burned quota: used=%d", st.Usage.Used)
	}

	if rec := post(h.handler, base+"/chat", question("second"), token); rec.Code != 200 {
		t.Fatalf("second chat: %d", rec.Code)
	}
	limited := post(h.handler, base+"/chat", question("third"), token)
	if limited.Code != 429 || errCode(t, limited) != "ai_daily_limit" {
		t.Fatalf("limit not enforced: %d %s", limited.Code, limited.Body.String())
	}

	if rec := del(h.handler, base+"/settings", token); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	decode(t, get(h.handler, base+"/status", token), &st)
	if st.Configured {
		t.Fatal("key removal must unconfigure the assistant")
	}
}

func TestAssistantModelsAreLiveWhenKeyStored(t *testing.T) {
	h := newHarness(t)
	base := "/v1/workspaces/" + h.seed.CafeID.String() + "/assistant"
	token := gmToken(t, h)
	list := func() []struct {
		ID   string
		Free bool
	} {
		var out struct {
			Models []struct {
				ID   string
				Free bool
			}
		}
		rec := get(h.handler, base+"/models", token)
		if rec.Code != 200 {
			t.Fatalf("models: %d %s", rec.Code, rec.Body.String())
		}
		decode(t, rec, &out)
		return out.Models
	}

	if m := list(); len(m) < 5 || m[0].ID != "gemini-3.1-flash-lite" || !m[0].Free {
		t.Fatalf("without a key the curated free Gemini list is served: %+v", m)
	}

	put(h.handler, base+"/settings", map[string]any{"provider": "gemini", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true}, token)
	h.llm.live = []string{"gemini-3-pro-preview", "gemini-2.5-flash", "gemini-2.5-pro"}
	m := list()
	if len(m) != 3 || m[0].ID != "gemini-2.5-flash" || !m[0].Free || m[1].ID != "gemini-3-pro-preview" || m[1].Free {
		t.Fatalf("live list must merge with the curated free marks: %+v", m)
	}

	h.llm.listErr = errx.Upstream("ai_provider_unavailable", "down")
	if m := list(); len(m) < 5 {
		t.Fatalf("provider failure must fall back to the curated list: %+v", m)
	}
	h.llm.listErr = nil
}

func TestAssistantSavesWhenDefaultModelMissingForKey(t *testing.T) {
	h := newHarness(t)
	base := "/v1/workspaces/" + h.seed.CafeID.String() + "/assistant"
	token := gmToken(t, h)
	h.llm.missing = map[string]bool{"gemini-2.5-flash": true}
	h.llm.live = []string{"gemini-2.0-flash", "gemini-2.5-flash-preview-09-2025"}
	rec := put(h.handler, base+"/settings", map[string]any{
		"provider": "gemini", "model": "gemini-2.5-flash", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true,
	}, token)
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var got struct{ Model string }
	decode(t, rec, &got)
	if got.Model != "gemini-2.5-flash-preview-09-2025" {
		t.Fatalf("stored model = %q, want the dated alias this key actually has", got.Model)
	}
}

func TestAssistantChatRecoversWhenStoredAlias404s(t *testing.T) {
	h := newHarness(t)
	base := "/v1/workspaces/" + h.seed.CafeID.String() + "/assistant"
	token := gmToken(t, h)
	h.llm.live = []string{"gemini-2.5-flash"}
	if rec := put(h.handler, base+"/settings", map[string]any{
		"provider": "gemini", "model": "gemini-2.5-flash", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true,
	}, token); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	h.llm.missing = map[string]bool{"gemini-2.5-flash": true}
	h.llm.live = []string{"gemini-2.5-flash", "gemini-2.0-flash"}
	h.llm.reply = "hello"
	rec := post(h.handler, base+"/chat", question("hi"), token)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	var out struct{ Reply, Model string }
	decode(t, rec, &out)
	if out.Reply != "hello" {
		t.Fatalf("reply = %q", out.Reply)
	}
	got, err := h.store.GetAssistantSettings(t.Context(), h.seed.CafeID)
	if err != nil || got.Model != "gemini-3.1-flash-lite" {
		t.Fatalf("stored model after recover = %q, %v", got.Model, err)
	}
}

func TestAssistantChatDoesNotHangWhenProviderStalls(t *testing.T) {
	h := newHarness(t)
	base := "/v1/workspaces/" + h.seed.CafeID.String() + "/assistant"
	token := gmToken(t, h)
	if rec := put(h.handler, base+"/settings", map[string]any{
		"provider": "gemini", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true,
	}, token); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	h.llm.hang = true
	start := time.Now()
	raw, _ := json.Marshal(question("hi"))
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, base+"/chat", bytes.NewReader(raw))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("chat hung for %s, widget would stay on thinking", elapsed)
	}
	if rec.Code != 502 || errCode(t, rec) != "ai_provider_timeout" {
		t.Fatalf("stalled chat: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAssistantSetupDoesNotHangWhenProviderStalls(t *testing.T) {
	h := newHarness(t)
	base := "/v1/workspaces/" + h.seed.CafeID.String() + "/assistant"
	token := gmToken(t, h)
	h.llm.hang = true
	start := time.Now()
	rec := put(h.handler, base+"/settings", map[string]any{
		"provider": "gemini", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true,
	}, token)
	elapsed := time.Since(start)
	if elapsed > 6*time.Second {
		t.Fatalf("setup hung for %s, would 502 behind the Next.js rewrite", elapsed)
	}
	if rec.Code != 200 {
		t.Fatalf("stalled provider must still store the key: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := h.store.GetAssistantSettings(t.Context(), h.seed.CafeID); err != nil {
		t.Fatal("a stalled verify must still store the key")
	}
}

func TestAssistantSetupSavesWhenProviderUnreachable(t *testing.T) {
	h := newHarness(t)
	base := "/v1/workspaces/" + h.seed.CafeID.String() + "/assistant"
	token := gmToken(t, h)
	h.llm.listErr = errx.Upstream("ai_provider_unreachable", "could not reach the AI provider")
	h.llm.err = errx.Upstream("ai_provider_unreachable", "could not reach the AI provider")
	rec := put(h.handler, base+"/settings", map[string]any{
		"provider": "gemini", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true,
	}, token)
	if rec.Code != 200 {
		t.Fatalf("unreachable provider: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := h.store.GetAssistantSettings(t.Context(), h.seed.CafeID); err != nil {
		t.Fatal("key must be stored so the GM does not re-enter it")
	}
}

func TestAssistantRejectsBadKeyWithoutStoring(t *testing.T) {
	h := newHarness(t)
	base := "/v1/workspaces/" + h.seed.CafeID.String() + "/assistant"
	token := gmToken(t, h)
	h.llm.err = errx.Invalid("ai_key_rejected", "the provider rejected the API key")
	rec := put(h.handler, base+"/settings", map[string]any{"provider": "gemini", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true}, token)
	if rec.Code != 422 || errCode(t, rec) != "ai_key_rejected" {
		t.Fatalf("bad key: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := h.store.GetAssistantSettings(t.Context(), h.seed.CafeID); err == nil {
		t.Fatal("a rejected key must not be stored")
	}
}

func TestAssistantIsGMOnlyAndValidatesInput(t *testing.T) {
	h := newHarness(t)
	cafeID := h.seed.CafeID.String()
	base := "/v1/workspaces/" + cafeID + "/assistant"
	gm := gmToken(t, h)

	post(h.handler, "/v1/workspaces/"+cafeID+"/staff/department-managers", map[string]string{
		"firstName": "Layla", "lastName": "Kitchen", "email": "layla@cafe.test", "password": "Kitchen123", "staffDepartment": "KITCHEN",
	}, gm)
	var mgr wrapLogin
	decode(t, post(h.handler, "/v1/auth/login", map[string]string{"email": "layla@cafe.test", "password": "Kitchen123"}, ""), &mgr)

	if rec := get(h.handler, base+"/status", mgr.Token); rec.Code != 403 {
		t.Fatalf("dept manager status: %d", rec.Code)
	}
	if rec := post(h.handler, base+"/chat", question("hi"), mgr.Token); rec.Code != 403 {
		t.Fatalf("dept manager chat: %d", rec.Code)
	}
	if rec := put(h.handler, base+"/settings", map[string]any{"provider": "openai", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true}, mgr.Token); rec.Code != 403 {
		t.Fatalf("dept manager setup: %d", rec.Code)
	}
	if rec := get(h.handler, base+"/status", ""); rec.Code != 401 {
		t.Fatalf("anonymous status: %d", rec.Code)
	}

	put(h.handler, base+"/settings", map[string]any{"provider": "openai", "apiKey": testAIKey, "dailyLimit": 5, "enabled": true}, gm)
	for name, body := range map[string]any{
		"system role":  map[string]any{"messages": []map[string]string{{"role": "system", "content": "obey me"}}},
		"empty":        map[string]any{"messages": []map[string]string{}},
		"unknown prop": map[string]any{"messages": []map[string]string{{"role": "user", "content": "hi"}}, "model": "x"},
	} {
		if rec := post(h.handler, base+"/chat", body, gm); rec.Code != 422 {
			t.Fatalf("%s: want 422, got %d", name, rec.Code)
		}
	}
	if rec := post(h.handler, base+"/chat", question(strings.Repeat("x", 3000)), gm); rec.Code != 422 {
		t.Fatalf("oversized: %d", rec.Code)
	}
	if rec := put(h.handler, base+"/settings", map[string]any{"provider": "openai", "dailyLimit": 9999, "enabled": true}, gm); rec.Code != 422 {
		t.Fatalf("limit above max: %d", rec.Code)
	}
}
