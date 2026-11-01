package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiThinking struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type geminiRequest struct {
	SystemInstruction geminiContent   `json:"systemInstruction"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  struct {
		MaxOutputTokens int             `json:"maxOutputTokens,omitempty"`
		Temperature     float64         `json:"temperature,omitempty"`
		ThinkingConfig  *geminiThinking `json:"thinkingConfig,omitempty"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (g *Gateway) gemini(ctx context.Context, req appassistant.CompletionRequest) (appassistant.CompletionResult, error) {
	body := geminiRequest{SystemInstruction: geminiContent{Parts: []geminiPart{{Text: req.System}}}}
	body.GenerationConfig.MaxOutputTokens = req.MaxOutputTokens
	body.GenerationConfig.Temperature = req.Temperature
	// 2.5 Flash thinks by default; 3.x lite rejects thinkingBudget=0.
	if thinkingBudget, ok := flashThinkingBudget(req.Model); ok {
		body.GenerationConfig.ThinkingConfig = &geminiThinking{ThinkingBudget: thinkingBudget}
	}
	for _, m := range req.Messages {
		role := "user"
		if m.Role == assistant.RoleAssistant {
			role = "model"
		}
		body.Contents = append(body.Contents, geminiContent{Role: role, Parts: []geminiPart{{Text: m.Content}}})
	}
	headers := map[string]string{"x-goog-api-key": req.APIKey}
	attempts := []geminiRequest{body}
	if body.GenerationConfig.ThinkingConfig != nil || body.GenerationConfig.Temperature != 0 {
		plain := body
		plain.GenerationConfig.ThinkingConfig = nil
		plain.GenerationConfig.Temperature = 0
		attempts = append(attempts, plain)
	}
	var last error
	for _, payload := range attempts {
		// Some keys 404 a model on v1beta but serve it on v1 (and the reverse).
		for _, ver := range []string{"v1beta", "v1"} {
			endpoint := strings.TrimRight(g.GeminiBase, "/") + "/" + ver + "/models/" + url.PathEscape(req.Model) + ":generateContent"
			data, err := g.post(ctx, endpoint, headers, payload, req.APIKey)
			if err == nil {
				return parseGemini(data)
			}
			last = err
			var xe *errx.Error
			if !errors.As(err, &xe) {
				return appassistant.CompletionResult{}, err
			}
			if xe.Code == "ai_model_not_found" {
				continue
			}
			if xe.Code == "ai_request_rejected" {
				break
			}
			return appassistant.CompletionResult{}, err
		}
	}
	return appassistant.CompletionResult{}, last
}

func flashThinkingBudget(model string) (int, bool) {
	lower := strings.ToLower(model)
	// Only 2.5 Flash thinks by default. 3.x lite rejects thinkingBudget=0.
	if strings.Contains(lower, "2.5") && strings.Contains(lower, "flash") && !strings.Contains(lower, "pro") {
		return 0, true
	}
	return 0, false
}

func parseGemini(data []byte) (appassistant.CompletionResult, error) {
	var out geminiResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return appassistant.CompletionResult{}, errx.Upstream("ai_provider_bad_response", "the AI provider sent an unreadable answer")
	}
	if len(out.Candidates) == 0 {
		return appassistant.CompletionResult{}, nil
	}
	var text strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		text.WriteString(p.Text)
	}
	return appassistant.CompletionResult{Text: strings.TrimSpace(text.String())}, nil
}
