package cafeintake

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
	GetIntake(ctx context.Context, workspaceID id.ID, businessDate string) (cafe.Intake, error)
	SaveIntake(ctx context.Context, rec cafe.Intake) error
	ListIntakes(ctx context.Context, workspaceID id.ID) ([]cafe.Intake, error)
}

type UserReader interface {
	GetUser(ctx context.Context, id id.ID) (identity.User, error)
}
