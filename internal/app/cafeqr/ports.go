package cafeqr

import (
	"context"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type DefinitionStore interface {
	GetCafeDefinition(ctx context.Context, workspaceID id.ID) (cafe.Definition, error)
	SaveCafeDefinition(ctx context.Context, def cafe.Definition) error
}

type CodeRepo interface {
	GetQRCodeBySite(ctx context.Context, workspaceID, siteID id.ID) (cafe.Code, error)
	GetQRCodeByToken(ctx context.Context, token string) (cafe.Code, error)
	SaveQRCode(ctx context.Context, code cafe.Code) error
	ListQRCodes(ctx context.Context, workspaceID id.ID) ([]cafe.Code, error)
}

type Inbox interface {
	CreateVoice(ctx context.Context, v cafe.Voice) error
	GetVoice(ctx context.Context, id id.ID) (cafe.Voice, error)
	ListVoices(ctx context.Context, workspaceID id.ID) ([]cafe.Voice, error)
	CountUnhappyVoicesSince(ctx context.Context, workspaceID id.ID, since time.Time) (int, error)
	GetRecoveryByVoice(ctx context.Context, voiceID id.ID) (cafe.Recovery, error)
	ListRecoveries(ctx context.Context, workspaceID id.ID) ([]cafe.Recovery, error)
	SaveRecovery(ctx context.Context, rec cafe.Recovery) error
}

// AssignmentPort is wired in bootstrap so this module does not import app/task (DIP).
type AssignmentPort interface {
	Assign(ctx context.Context, ac actor.Context, title, description, assigneeUserID string, start, end time.Time) (id.ID, error)
}
