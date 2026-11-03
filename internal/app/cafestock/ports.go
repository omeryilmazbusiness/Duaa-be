package cafestock

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type DefinitionStore interface {
	GetCafeDefinition(ctx context.Context, workspaceID id.ID) (cafe.Definition, error)
}

type Repository interface {
	GetStockCatalog(ctx context.Context, workspaceID id.ID) (cafe.StockCatalog, error)
	SaveStockCatalog(ctx context.Context, rec cafe.StockCatalog) error
	CreateStockAdjustment(ctx context.Context, adj cafe.StockAdjustment) error
	ListStockAdjustments(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.StockAdjustment, error)
}
