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

type AudioTranscribeRequest struct {
	Provider     assistant.Provider
	Model        string
	APIKey       string
	Audio        []byte
	MimeType     string
	FileName     string
	LanguageHint string
}

type AudioTranscribeResult struct {
	Text             string
	DetectedLanguage string
}

type AudioSpeakRequest struct {
	Provider assistant.Provider
	APIKey   string
	Text     string
	Language string
}

// AudioGateway is optional: provides Whisper/Gemini audio transcription and OpenAI TTS.
type AudioGateway interface {
	Transcribe(ctx context.Context, req AudioTranscribeRequest) (AudioTranscribeResult, error)
	Speak(ctx context.Context, req AudioSpeakRequest) (audio []byte, contentType string, err error)
}

type MeetingRepo interface {
	SaveMeeting(ctx context.Context, rec assistant.MeetingRecord) error
	ListMeetings(ctx context.Context, workspaceID id.ID, limit int) ([]assistant.MeetingRecord, error)
}
