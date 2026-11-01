package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const maxModelPages = 5

// ListModels returns the chat-capable model ids a key can use. A successful
// list also proves the key; OpenAI is included so setup does not need a
// generate call that can hang the Next.js rewrite.
func (g *Gateway) ListModels(ctx context.Context, provider assistant.Provider, apiKey string) ([]string, error) {
	switch provider {
	case assistant.ProviderGemini:
		return g.listGemini(ctx, apiKey)
	case assistant.ProviderOpenAI:
		return g.listOpenAI(ctx, apiKey)
	default:
		return nil, nil
	}
}

func (g *Gateway) listGemini(ctx context.Context, apiKey string) ([]string, error) {
	var ids []string
	token := ""
	for page := 0; page < maxModelPages; page++ {
		endpoint := strings.TrimRight(g.GeminiBase, "/") + "/v1beta/models?pageSize=200"
		if token != "" {
			endpoint += "&pageToken=" + url.QueryEscape(token)
		}
		data, err := g.do(ctx, http.MethodGet, endpoint, map[string]string{"x-goog-api-key": apiKey}, nil, apiKey)
		if err != nil {
			return nil, err
		}
		var out struct {
			Models []struct {
				Name    string   `json:"name"`
				Methods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, errx.Upstream("ai_provider_bad_response", "the AI provider sent an unreadable answer")
		}
		for _, m := range out.Models {
			id := strings.TrimPrefix(m.Name, "models/")
			if strings.HasPrefix(id, "gemini-") && supportsGenerate(m.Methods) {
				ids = append(ids, id)
			}
		}
		if out.NextPageToken == "" {
			break
		}
		token = out.NextPageToken
	}
	return ids, nil
}

func (g *Gateway) listOpenAI(ctx context.Context, apiKey string) ([]string, error) {
	data, err := g.do(ctx, http.MethodGet, strings.TrimRight(g.OpenAIBase, "/")+"/v1/models",
		map[string]string{"Authorization": "Bearer " + apiKey}, nil, apiKey)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, errx.Upstream("ai_provider_bad_response", "the AI provider sent an unreadable answer")
	}
	var ids []string
	for _, m := range out.Data {
		if strings.HasPrefix(m.ID, "gpt-") && !strings.Contains(m.ID, "instruct") {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

func supportsGenerate(methods []string) bool {
	for _, m := range methods {
		if m == "generateContent" {
			return true
		}
	}
	return false
}
