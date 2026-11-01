package cafedef

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// Repository is the one-document store for a cafe workspace.
type Repository interface {
	GetCafeDefinition(ctx context.Context, workspaceID id.ID) (cafe.Definition, error)
	SaveCafeDefinition(ctx context.Context, def cafe.Definition) error
}

// WorkspaceWriter syncs official name and timezone onto the tenancy record.
type WorkspaceWriter interface {
	GetWorkspace(ctx context.Context, id id.ID) (tenancy.Workspace, error)
	UpdateWorkspace(ctx context.Context, w tenancy.Workspace) error
}
