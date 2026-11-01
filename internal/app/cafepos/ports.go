package cafepos

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type DefinitionStore interface {
	GetCafeDefinition(ctx context.Context, workspaceID id.ID) (cafe.Definition, error)
}

type Repository interface {
	CreatePOSTicket(ctx context.Context, t cafe.Ticket) error
	ListPOSTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.Ticket, error)
	GetPOSClose(ctx context.Context, workspaceID id.ID, businessDate string) (cafe.POSClose, error)
	CreatePOSClose(ctx context.Context, c cafe.POSClose) error
}
