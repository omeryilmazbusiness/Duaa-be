package cafehandover

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type DefinitionStore interface {
	GetCafeDefinition(ctx context.Context, workspaceID id.ID) (cafe.Definition, error)
}

type Repository interface {
	GetOpenHandover(ctx context.Context, workspaceID id.ID) (cafe.Handover, error)
	GetHandover(ctx context.Context, handoverID id.ID) (cafe.Handover, error)
	SaveHandover(ctx context.Context, rec cafe.Handover) error
	ListHandovers(ctx context.Context, workspaceID id.ID) ([]cafe.Handover, error)
}

// OpenWork is a read-model of live floor files. Implementations must not invent titles.
type OpenWork interface {
	SnapshotOpenWork(ctx context.Context, workspaceID id.ID) (tasks, recoveries, breakdowns []cafe.HandoverRef, err error)
}

type UserReader interface {
	GetUser(ctx context.Context, id id.ID) (identity.User, error)
}
