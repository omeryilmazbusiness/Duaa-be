package dashboard

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type TaskReader interface {
	ListTasksByWorkspace(ctx context.Context, workspaceID id.ID) ([]work.Task, error)
}

type MembershipReader interface {
	ListWorkspaceMemberships(ctx context.Context, workspaceID id.ID) ([]identity.Membership, error)
}

type ReportReader interface {
	ListReports(ctx context.Context, workspaceID id.ID) ([]opsreport.Report, error)
}

type UserReader interface {
	GetUser(ctx context.Context, id id.ID) (identity.User, error)
}

type CashReader interface {
	GetCloseByDate(ctx context.Context, workspaceID id.ID, businessDate string) (finance.Close, error)
}

type ApprovalReader interface {
	ListApprovals(ctx context.Context, workspaceID id.ID) ([]finance.Approval, error)
}

type SalesReader interface {
	ListPOSTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.Ticket, error)
	GetPOSClose(ctx context.Context, workspaceID id.ID, businessDate string) (cafe.POSClose, error)
}
