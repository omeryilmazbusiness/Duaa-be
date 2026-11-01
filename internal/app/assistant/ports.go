package assistant

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// SettingsRepo persists one assistant configuration per workspace.
type SettingsRepo interface {
	GetAssistantSettings(ctx context.Context, workspaceID id.ID) (assistant.Settings, error)
	SaveAssistantSettings(ctx context.Context, s assistant.Settings) error
	DeleteAssistantSettings(ctx context.Context, workspaceID id.ID) error
}

// UsageRepo counts assistant requests per workspace and business date.
// ReserveAIUsage must be atomic so concurrent chats can never exceed limit.
type UsageRepo interface {
	ReserveAIUsage(ctx context.Context, workspaceID id.ID, date string, limit int) (used int, ok bool, err error)
	ReleaseAIUsage(ctx context.Context, workspaceID id.ID, date string) error
	AIUsage(ctx context.Context, workspaceID id.ID, date string) (int, error)
}

// Cipher seals and opens the provider API key.
type Cipher interface {
	Seal(plaintext, aad string) (string, error)
	Open(sealed, aad string) (string, error)
}

type CompletionRequest struct {
	Provider        assistant.Provider
	Model           string
	APIKey          string
	System          string
	Messages        []assistant.Message
	MaxOutputTokens int
	Temperature     float64
}

type CompletionResult struct{ Text string }

// Completer talks to the external model provider.
type Completer interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error)
}

// ContextProvider renders a read-only text snapshot of the workspace.
type ContextProvider interface {
	Snapshot(ctx context.Context, ac actor.Context) (string, error)
}

// ModelLister asks the provider which models a key can use. It is optional:
// when it fails the service falls back to the built-in catalogue.
type ModelLister interface {
	ListModels(ctx context.Context, provider assistant.Provider, apiKey string) ([]string, error)
}
