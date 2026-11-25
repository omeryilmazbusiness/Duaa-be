package inbox

import (
	"context"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/inbox"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	GetConnection(ctx context.Context, workspaceID id.ID) (inbox.Connection, error)
	SaveConnection(ctx context.Context, c inbox.Connection) error
	DeleteConnection(ctx context.Context, workspaceID id.ID) error
	UpsertMessage(ctx context.Context, m inbox.Message) error
	GetMessage(ctx context.Context, workspaceID, messageID id.ID) (inbox.Message, error)
	ListMessages(ctx context.Context, workspaceID id.ID, limit int) ([]inbox.Message, error)
	CountUnread(ctx context.Context, workspaceID id.ID) (int, error)
	DeleteMessages(ctx context.Context, workspaceID id.ID) error
}

type Cipher interface {
	Seal(plaintext, aad string) (string, error)
	Open(sealed, aad string) (string, error)
}

type SettingsRepo interface {
	GetAssistantSettings(ctx context.Context, workspaceID id.ID) (assistant.Settings, error)
}

type UsageRepo interface {
	ReserveAIUsage(ctx context.Context, workspaceID id.ID, businessDate string, limit int) (used int, ok bool, err error)
	ReleaseAIUsage(ctx context.Context, workspaceID id.ID, businessDate string) error
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

type CompletionResult struct {
	Text string
}

type Completer interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error)
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scopes       string
}

type Profile struct {
	Email       string
	DisplayName string
}

type ClientFactory func(clientID, clientSecret, tenant string) MailClient

type MailClient interface {
	Configured() bool
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (TokenPair, Profile, error)
	Refresh(ctx context.Context, refreshToken string) (TokenPair, error)
	ListInbox(ctx context.Context, accessToken string) ([]inbox.RemoteMessage, error)
	Send(ctx context.Context, accessToken string, draft inbox.Draft) error
}

type DemoMailbox interface {
	Connect(ctx context.Context, workspaceID id.ID) (inbox.Connection, []inbox.RemoteMessage, error)
	Send(ctx context.Context, workspaceID id.ID, draft inbox.Draft, fromEmail string) error
}

type Clock interface {
	Now() time.Time
}
