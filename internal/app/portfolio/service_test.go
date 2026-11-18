package portfolio_test

import (
	"context"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/portfolio"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/supply"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type stubRepo struct {
	memberships []identity.Membership
	spaces      map[id.ID]tenancy.Workspace
	pms         map[string]hotel.PMSSnapshot
	tickets     []cafe.Ticket
}

func (s *stubRepo) ListMembershipsByUser(_ context.Context, uid id.ID) ([]identity.Membership, error) {
	var out []identity.Membership
	for _, m := range s.memberships {
		if m.UserID == uid {
			out = append(out, m)
		}
	}
	return out, nil
}
func (s *stubRepo) GetWorkspace(_ context.Context, wid id.ID) (tenancy.Workspace, error) {
	w, ok := s.spaces[wid]
	if !ok {
		return tenancy.Workspace{}, errx.NotFound("not_found", "missing")
	}
	return w, nil
}
func (s *stubRepo) ListTasksByWorkspace(_ context.Context, _ id.ID) ([]work.Task, error) { return nil, nil }
func (s *stubRepo) ListReports(_ context.Context, _ id.ID) ([]opsreport.Report, error) {
	return nil, nil
}
func (s *stubRepo) GetCloseByDate(_ context.Context, _ id.ID, _ string) (finance.Close, error) {
	return finance.Close{}, errx.NotFound("not_found", "missing")
}
func (s *stubRepo) ListApprovals(_ context.Context, _ id.ID) ([]finance.Approval, error) {
	return nil, nil
}
func (s *stubRepo) GetOHIPConfig(_ context.Context, _ id.ID) (hotel.OHIPConfig, error) {
	return hotel.OHIPConfig{}, hotel.OHIPConfigNotFoundError()
}
func (s *stubRepo) GetPMSSnapshot(_ context.Context, wid id.ID, date string) (hotel.PMSSnapshot, error) {
	snap, ok := s.pms[wid.String()+":"+date]
	if !ok {
		return hotel.PMSSnapshot{}, hotel.PMSSnapshotNotFoundError()
	}
	return snap, nil
}
func (s *stubRepo) ListPOSTickets(_ context.Context, wid id.ID, date string) ([]cafe.Ticket, error) {
	var out []cafe.Ticket
	for _, tk := range s.tickets {
		if tk.WorkspaceID == wid && tk.BusinessDate == date {
			out = append(out, tk)
		}
	}
	return out, nil
}
func (s *stubRepo) GetPOSClose(_ context.Context, _ id.ID, _ string) (cafe.POSClose, error) {
	return cafe.POSClose{}, errx.NotFound("not_found", "missing")
}
func (s *stubRepo) ListWasteTickets(_ context.Context, _ id.ID, _ string) ([]cafe.WasteTicket, error) {
	return nil, nil
}
func (s *stubRepo) GetStockCatalog(_ context.Context, _ id.ID) (cafe.StockCatalog, error) {
	return cafe.StockCatalog{}, cafe.StockCatalogNotFoundError()
}
func (s *stubRepo) GetChecklistRun(_ context.Context, _ id.ID, _ string, _ cafe.CheckKind) (cafe.ChecklistRun, error) {
	return cafe.ChecklistRun{}, errx.NotFound("not_found", "missing")
}
func (s *stubRepo) GetOpenHandover(_ context.Context, _ id.ID) (cafe.Handover, error) {
	return cafe.Handover{}, errx.NotFound("not_found", "missing")
}
func (s *stubRepo) GetIntake(_ context.Context, _ id.ID, _ string) (cafe.Intake, error) {
	return cafe.Intake{}, errx.NotFound("not_found", "missing")
}
func (s *stubRepo) ListRecoveries(_ context.Context, _ id.ID) ([]cafe.Recovery, error) {
	return nil, nil
}
func (s *stubRepo) ListDocuments(_ context.Context, _ id.ID, _ int) ([]docintake.Document, error) {
	return nil, nil
}

func TestPortfolioAggregatesTwoHotelsAndCafeAndSeparatesCurrencies(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	gmID := id.New()
	cafeID := id.New()
	sheratonID := id.New()
	shohadaID := id.New()
	today := "2026-11-18"
	yday := "2026-11-17"

	repo := &stubRepo{
		memberships: []identity.Membership{
			{ID: id.New(), WorkspaceID: cafeID, UserID: gmID, Role: identity.RoleManager, IsActive: true},
			{ID: id.New(), WorkspaceID: sheratonID, UserID: gmID, Role: identity.RoleManager, IsActive: true},
			{ID: id.New(), WorkspaceID: shohadaID, UserID: gmID, Role: identity.RoleManager, IsActive: true},
		},
		spaces: map[id.ID]tenancy.Workspace{
			cafeID:     {ID: cafeID, Kind: tenancy.KindCafe, Name: "Duaa Cafe", Slug: "duaa-cafe", Timezone: "UTC", IsActive: true},
			sheratonID: {ID: sheratonID, Kind: tenancy.KindHotel, Name: "Sheraton Hotel", Slug: "sheraton-hotel", Timezone: "UTC", IsActive: true},
			shohadaID:  {ID: shohadaID, Kind: tenancy.KindHotel, Name: "Al Shohada Hotel", Slug: "al-shohada-hotel", Timezone: "UTC", IsActive: true},
		},
		pms: map[string]hotel.PMSSnapshot{
			sheratonID.String() + ":" + yday: {
				ID: id.New(), WorkspaceID: sheratonID, BusinessDate: yday, Source: hotel.SourceOPERAExport,
				Currency: "USD", TotalRooms: 100, OccupiedRooms: 70, RoomRevenueMinor: 20000_00, CapturedAt: now.Add(-24 * time.Hour), RecordedBy: gmID,
			},
			sheratonID.String() + ":" + today: {
				ID: id.New(), WorkspaceID: sheratonID, BusinessDate: today, Source: hotel.SourceOHIPLive,
				Currency: "USD", TotalRooms: 100, OccupiedRooms: 80, RoomRevenueMinor: 25000_00, CapturedAt: now, RecordedBy: gmID,
			},
			shohadaID.String() + ":" + yday: {
				ID: id.New(), WorkspaceID: shohadaID, BusinessDate: yday, Source: hotel.SourceOPERAExport,
				Currency: "USD", TotalRooms: 100, OccupiedRooms: 50, RoomRevenueMinor: 10000_00, CapturedAt: now.Add(-24 * time.Hour), RecordedBy: gmID,
			},
			shohadaID.String() + ":" + today: {
				ID: id.New(), WorkspaceID: shohadaID, BusinessDate: today, Source: hotel.SourceOPERAExport,
				Currency: "USD", TotalRooms: 100, OccupiedRooms: 60, RoomRevenueMinor: 15000_00, CapturedAt: now, RecordedBy: gmID,
			},
		},
	}
	ticket, err := cafe.NewTicket(cafeID, gmID, today, "POS-1", "sale", "cash", 2, 120000, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	repo.tickets = append(repo.tickets, ticket)

	reg := tenancy.NewRegistry(cafe.Catalog{}, hotel.Catalog{}, supply.Catalog{})
	svc := portfolio.New(repo, reg, clock.Frozen{T: now})

	view, err := svc.Get(t.Context(), ctxkey.Principal{UserID: gmID.String(), WorkspaceID: cafeID.String(), Role: "manager"}, today)
	if err != nil {
		t.Fatalf("portfolio err=%v", err)
	}
	if view.Hotels.Count != 2 || view.Cafes.Count != 1 {
		t.Fatalf("counts hotels=%d cafes=%d", view.Hotels.Count, view.Cafes.Count)
	}
	if view.Hotels.AvgOccupancyPct == nil || *view.Hotels.AvgOccupancyPct != 70.0 {
		t.Fatalf("avg occ=%v want 70.0", view.Hotels.AvgOccupancyPct)
	}
	if len(view.Hotels.TotalsByCurrency) != 1 || view.Hotels.TotalsByCurrency[0].TodayTotalRevenueMinor != 40000_00 || view.Hotels.TotalsByCurrency[0].DeltaMinor != 10000_00 {
		t.Fatalf("currency totals=%+v", view.Hotels.TotalsByCurrency)
	}
	if view.Cafes.Cafes[0].SalesNet != 120000 || view.Cafes.Cafes[0].CompletenessPct <= 0 {
		t.Fatalf("cafe=%+v", view.Cafes.Cafes[0])
	}
}
