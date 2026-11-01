package cash

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	GetCloseByDate(ctx context.Context, workspaceID id.ID, businessDate string) (finance.Close, error)
	CreateClose(ctx context.Context, c finance.Close) error
	UpdateClose(ctx context.Context, c finance.Close) error
}
