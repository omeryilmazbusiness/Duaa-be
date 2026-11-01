package report

import (
	"context"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	CreateReport(ctx context.Context, r opsreport.Report) error
	GetReport(ctx context.Context, id id.ID) (opsreport.Report, error)
	ListReports(ctx context.Context, workspaceID id.ID) ([]opsreport.Report, error)
	UpdateReport(ctx context.Context, r opsreport.Report) error
}

type MembershipReader interface {
	ListWorkspaceMemberships(ctx context.Context, workspaceID id.ID) ([]identity.Membership, error)
}

type UserReader interface {
	GetUser(ctx context.Context, id id.ID) (identity.User, error)
}

// AssignmentPort is implemented in bootstrap so this module does not import app/task (DIP).
type AssignmentPort interface {
	Assign(ctx context.Context, ac actor.Context, title, description, assigneeUserID string, start, end time.Time) (id.ID, error)
}
