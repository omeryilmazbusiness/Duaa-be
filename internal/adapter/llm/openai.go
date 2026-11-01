package llm

import (
	"context"
	"encoding/json"
	"strings"

	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Temperature         float64         `json:"temperature,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (g *Gateway) openAI(ctx context.Context, req appassistant.CompletionRequest) (appassistant.CompletionResult, error) {
	msgs := make([]openAIMessage, 0, len(req.Messages)+1)
	msgs = append(msgs, openAIMessage{Role: "system", Content: req.System})
	for _, m := range req.Messages {
		msgs = append(msgs, openAIMessage{Role: string(m.Role), Content: m.Content})
	}
	body := openAIRequest{Model: req.Model, Messages: msgs, MaxCompletionTokens: req.MaxOutputTokens, Temperature: req.Temperature}
	data, err := g.post(ctx, strings.TrimRight(g.OpenAIBase, "/")+"/v1/chat/completions",
		map[string]string{"Authorization": "Bearer " + req.APIKey}, body, req.APIKey)
	if err != nil {
		return appassistant.CompletionResult{}, err
	}
	var out openAIResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return appassistant.CompletionResult{}, errx.Upstream("ai_provider_bad_response", "the AI provider sent an unreadable answer")
	}
	if len(out.Choices) == 0 {
		return appassistant.CompletionResult{}, nil
	}
	return appassistant.CompletionResult{Text: strings.TrimSpace(out.Choices[0].Message.Content)}, nil
}
