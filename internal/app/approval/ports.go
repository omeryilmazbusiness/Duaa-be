package approval

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	CreateApproval(ctx context.Context, a finance.Approval) error
	GetApproval(ctx context.Context, id id.ID) (finance.Approval, error)
	ListApprovals(ctx context.Context, workspaceID id.ID) ([]finance.Approval, error)
	UpdateApproval(ctx context.Context, a finance.Approval) error
}
