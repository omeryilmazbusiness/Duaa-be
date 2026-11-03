package cafewaste

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type DefinitionStore interface {
	GetCafeDefinition(ctx context.Context, workspaceID id.ID) (cafe.Definition, error)
}

type Repository interface {
	CreateWasteTicket(ctx context.Context, t cafe.WasteTicket) error
	ListWasteTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.WasteTicket, error)
	GetRecipeBook(ctx context.Context, workspaceID id.ID) (cafe.RecipeBook, error)
	SaveRecipeBook(ctx context.Context, rec cafe.RecipeBook) error
}
