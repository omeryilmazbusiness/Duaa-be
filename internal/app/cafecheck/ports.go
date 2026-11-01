package cafecheck

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
	GetChecklistTemplate(ctx context.Context, workspaceID id.ID) (cafe.ChecklistTemplate, error)
	SaveChecklistTemplate(ctx context.Context, rec cafe.ChecklistTemplate) error
	GetChecklistRun(ctx context.Context, workspaceID id.ID, businessDate string, kind cafe.CheckKind) (cafe.ChecklistRun, error)
	SaveChecklistRun(ctx context.Context, rec cafe.ChecklistRun) error
}

type UserReader interface {
	GetUser(ctx context.Context, id id.ID) (identity.User, error)
}
