package portfolio

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	ListMembershipsByUser(ctx context.Context, userID id.ID) ([]identity.Membership, error)
	GetWorkspace(ctx context.Context, id id.ID) (tenancy.Workspace, error)
	ListTasksByWorkspace(ctx context.Context, workspaceID id.ID) ([]work.Task, error)
	ListReports(ctx context.Context, workspaceID id.ID) ([]opsreport.Report, error)
	GetCloseByDate(ctx context.Context, workspaceID id.ID, businessDate string) (finance.Close, error)
	ListApprovals(ctx context.Context, workspaceID id.ID) ([]finance.Approval, error)
	GetOHIPConfig(ctx context.Context, workspaceID id.ID) (hotel.OHIPConfig, error)
	GetPMSSnapshot(ctx context.Context, workspaceID id.ID, businessDate string) (hotel.PMSSnapshot, error)
	ListPOSTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.Ticket, error)
	GetPOSClose(ctx context.Context, workspaceID id.ID, businessDate string) (cafe.POSClose, error)
	ListWasteTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.WasteTicket, error)
	GetStockCatalog(ctx context.Context, workspaceID id.ID) (cafe.StockCatalog, error)
	GetChecklistRun(ctx context.Context, workspaceID id.ID, businessDate string, kind cafe.CheckKind) (cafe.ChecklistRun, error)
	GetOpenHandover(ctx context.Context, workspaceID id.ID) (cafe.Handover, error)
	GetIntake(ctx context.Context, workspaceID id.ID, businessDate string) (cafe.Intake, error)
	ListRecoveries(ctx context.Context, workspaceID id.ID) ([]cafe.Recovery, error)
	ListDocuments(ctx context.Context, workspaceID id.ID, limit int) ([]docintake.Document, error)
}
