package cafeprep

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type DefinitionStore interface {
	GetCafeDefinition(ctx context.Context, workspaceID id.ID) (cafe.Definition, error)
}

type Repository interface {
	CreatePrepTicket(ctx context.Context, t cafe.PrepTicket) error
	GetPrepTicket(ctx context.Context, id id.ID) (cafe.PrepTicket, error)
	ListPrepTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.PrepTicket, error)
	SavePrepTicket(ctx context.Context, t cafe.PrepTicket) error
}
