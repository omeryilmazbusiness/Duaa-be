// Package llm adapts the assistant's Completer port to the OpenAI and Gemini
// HTTP APIs. API keys only ever live in memory for the duration of a call and
// are redacted from any provider text that is surfaced to users.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const (
	defaultOpenAIBase = "https://api.openai.com"
	defaultGeminiBase = "https://generativelanguage.googleapis.com"
	maxResponseBytes  = 1 << 20
)

type Gateway struct {
	HTTP       *http.Client
	OpenAIBase string
	GeminiBase string
}

// NewGateway uses the public provider endpoints unless a base URL override is
// given (an operator-set proxy or gateway; never user input).
func NewGateway(openAIBase, geminiBase string) *Gateway {
	if openAIBase == "" {
		openAIBase = defaultOpenAIBase
	}
	if geminiBase == "" {
		geminiBase = defaultGeminiBase
	}
	return &Gateway{
		HTTP:       newHTTPClient(),
		OpenAIBase: openAIBase,
		GeminiBase: geminiBase,
	}
}

func newHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 25 * time.Second,
	}
	return &http.Client{Timeout: 50 * time.Second, Transport: transport}
}

var (
	_ appassistant.Completer   = (*Gateway)(nil)
	_ appassistant.ModelLister = (*Gateway)(nil)
)

func (g *Gateway) Complete(ctx context.Context, req appassistant.CompletionRequest) (appassistant.CompletionResult, error) {
	switch req.Provider {
	case assistant.ProviderOpenAI:
		return g.openAI(ctx, req)
	case assistant.ProviderGemini:
		return g.gemini(ctx, req)
	default:
		return appassistant.CompletionResult{}, errx.Invalid("ai_provider_invalid", "unsupported provider")
	}
}

// post sends a JSON request and returns the body of a 2xx answer. Every other
// outcome is translated into a safe, user-presentable errx error.
func (g *Gateway) post(ctx context.Context, url string, headers map[string]string, body any, secret string) ([]byte, error) {
	return g.do(ctx, http.MethodPost, url, headers, body, secret)
}

// do sends one request (JSON body when body != nil) and returns a 2xx body.
func (g *Gateway) do(ctx context.Context, method, url string, headers map[string]string, body any, secret string) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return nil, transportError(ctx, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, transportError(ctx, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return data, nil
	}
	return nil, statusError(resp.StatusCode, data, secret)
}

func transportError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return errx.Upstream("ai_provider_timeout", "the AI provider took too long to answer; try again")
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	return errx.Upstream("ai_provider_unreachable", "could not reach the AI provider; try again shortly")
}

func statusError(status int, body []byte, secret string) error {
	msg := providerMessage(body, secret)
	lower := strings.ToLower(msg)
	if regionBlocked(lower) {
		return errx.Invalid("ai_region_blocked", "this AI provider is not available from the current location")
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return errx.Invalid("ai_key_rejected", "the provider rejected the API key (invalid, revoked or without access)")
	case status == http.StatusNotFound:
		return errx.Invalid("ai_model_not_found", "the model was not found for this provider and key")
	case status == http.StatusTooManyRequests:
		return errx.RateLimited("ai_provider_rate_limited", "the AI provider is rate limiting this key or its quota is used up")
	case status >= 500:
		return errx.Upstream("ai_provider_unavailable", "the AI provider is having problems; try again shortly")
	}
	if strings.Contains(lower, "api key") {
		return errx.Invalid("ai_key_rejected", "the provider rejected the API key (invalid, revoked or without access)")
	}
	if strings.Contains(lower, "not found") || strings.Contains(lower, "is not supported") {
		return errx.Invalid("ai_model_not_found", "the model was not found for this provider and key")
	}
	if msg == "" {
		msg = "the provider rejected the request"
	}
	return errx.Invalid("ai_request_rejected", "the provider rejected the request: "+msg)
}

func regionBlocked(lower string) bool {
	return strings.Contains(lower, "user location is not supported") ||
		strings.Contains(lower, "location is not supported") ||
		strings.Contains(lower, "failed precondition") && strings.Contains(lower, "location")
}

// providerMessage extracts error.message from the common provider envelope,
// redacting the key and capping the length.
func providerMessage(body []byte, secret string) string {
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &env) != nil {
		return ""
	}
	msg := strings.Join(strings.Fields(env.Error.Message), " ")
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "[redacted]")
	}
	if r := []rune(msg); len(r) > 200 {
		msg = string(r[:200]) + "…"
	}
	return msg
}
