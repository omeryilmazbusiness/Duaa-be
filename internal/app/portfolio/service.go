package portfolio

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	repo     Repository
	registry *tenancy.Registry
	clock    clock.Clock
}

func New(repo Repository, registry *tenancy.Registry, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, registry: registry, clock: clk}
}

type ChannelStatus struct {
	Key       string `json:"key"`
	Entered   bool   `json:"entered"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type WorkspaceCompleteness struct {
	WorkspaceID   string          `json:"workspaceId"`
	WorkspaceName string          `json:"workspaceName"`
	Kind          string          `json:"kind"`
	EnteredCount  int             `json:"enteredCount"`
	ExpectedCount int             `json:"expectedCount"`
	Pct           int             `json:"pct"`
	Channels      []ChannelStatus `json:"channels"`
}

type CompletenessSummary struct {
	OverallPct  int                     `json:"overallPct"`
	ByWorkspace []WorkspaceCompleteness `json:"byWorkspace"`
}

type CurrencyTotal struct {
	Currency               string   `json:"currency"`
	HotelCount             int      `json:"hotelCount"`
	TodayTotalRevenueMinor int64    `json:"todayTotalRevenueMinor"`
	TodayRoomRevenueMinor  int64    `json:"todayRoomRevenueMinor"`
	YesterdayRevenueMinor  int64    `json:"yesterdayRevenueMinor"`
	HasYesterday           bool     `json:"hasYesterday"`
	DeltaMinor             int64    `json:"deltaMinor"`
	DeltaPct               *float64 `json:"deltaPct,omitempty"`
}

type HotelCard struct {
	WorkspaceID           string   `json:"workspaceId"`
	Name                  string   `json:"name"`
	Slug                  string   `json:"slug"`
	BusinessDate          string   `json:"businessDate"`
	Freshness             string   `json:"freshness"`
	Source                string   `json:"source,omitempty"`
	Currency              string   `json:"currency,omitempty"`
	TotalRooms            int      `json:"totalRooms"`
	AvailableRooms        int      `json:"availableRooms"`
	OccupiedRooms         int      `json:"occupiedRooms"`
	OutOfOrderRooms       int      `json:"outOfOrderRooms"`
	OccupancyPct          *float64 `json:"occupancyPct,omitempty"`
	RoomRevenueMinor      int64    `json:"roomRevenueMinor"`
	FBRevenueMinor        int64    `json:"fbRevenueMinor"`
	OtherRevenueMinor     int64    `json:"otherRevenueMinor"`
	TotalRevenueMinor     int64    `json:"totalRevenueMinor"`
	ADRMinor              int64    `json:"adrMinor"`
	RevPARMinor           int64    `json:"revparMinor"`
	ArrivalsCount         int      `json:"arrivalsCount"`
	DeparturesCount       int      `json:"departuresCount"`
	ReservationsCount     int      `json:"reservationsCount"`
	YesterdayRevenueMinor int64    `json:"yesterdayRevenueMinor"`
	HasYesterday          bool     `json:"hasYesterday"`
	RevenueDeltaMinor     int64    `json:"revenueDeltaMinor"`
	RevenueDeltaPct       *float64 `json:"revenueDeltaPct,omitempty"`
	ReadinessPct          int      `json:"readinessPct"`
	CompletenessPct       int      `json:"completenessPct"`
	OpenTasks             int      `json:"openTasks"`
	OverdueTasks          int      `json:"overdueTasks"`
	OpenReports           int      `json:"openReports"`
	OHIPConfigured        bool     `json:"ohipConfigured"`
	OHOPLastSyncStatus    string   `json:"ohipLastSyncStatus"`
}

type HotelsSummary struct {
	Count            int             `json:"count"`
	ConnectedCount   int             `json:"connectedCount"`
	AvgOccupancyPct  *float64        `json:"avgOccupancyPct,omitempty"`
	TotalRooms       int             `json:"totalRooms"`
	AvailableRooms   int             `json:"availableRooms"`
	OccupiedRooms    int             `json:"occupiedRooms"`
	OutOfOrderRooms  int             `json:"outOfOrderRooms"`
	TotalArrivals    int             `json:"totalArrivals"`
	TotalDepartures  int             `json:"totalDepartures"`
	TotalsByCurrency []CurrencyTotal `json:"totalsByCurrency"`
	Hotels           []HotelCard     `json:"hotels"`
}

type CafeProfitability struct {
	Status                string   `json:"status"` // live | provisional | unavailable
	Currency              string   `json:"currency"`
	SalesNet              int64    `json:"salesNet"`
	WasteCost             int64    `json:"wasteCost"`
	ApprovedExpenses      int64    `json:"approvedExpenses"`
	PendingApprovalsCount int      `json:"pendingApprovalsCount"`
	EstimatedNet          int64    `json:"estimatedNet"`
	MarginPct             *float64 `json:"marginPct,omitempty"`
	Complete              bool     `json:"complete"`
}

type CafeCard struct {
	WorkspaceID     string            `json:"workspaceId"`
	Name            string            `json:"name"`
	Slug            string            `json:"slug"`
	BusinessDate    string            `json:"businessDate"`
	SalesStatus     string            `json:"salesStatus"`
	SalesNet        int64             `json:"salesNet"`
	SalesGross      int64             `json:"salesGross"`
	TicketCount     int               `json:"ticketCount"`
	POSClosed       bool              `json:"posClosed"`
	WasteStatus     string            `json:"wasteStatus"`
	WasteCost       int64             `json:"wasteCost"`
	WasteGrams      int64             `json:"wasteGrams"`
	StockStatus     string            `json:"stockStatus"`
	StockItems      int               `json:"stockItems"`
	StockLowCount   int               `json:"stockLowCount"`
	CashStatus      string            `json:"cashStatus"`
	CashVariance    int64             `json:"cashVariance"`
	Profitability   CafeProfitability `json:"profitability"`
	CompletenessPct int               `json:"completenessPct"`
	OpenTasks       int               `json:"openTasks"`
	OverdueTasks    int               `json:"overdueTasks"`
	OpenReports     int               `json:"openReports"`
	OpenRecoveries  int               `json:"openRecoveries"`
}

type CafeSummary struct {
	Count int        `json:"count"`
	Cafes []CafeCard `json:"cafes"`
}

type RecentDocument struct {
	ID              string                    `json:"id"`
	WorkspaceID     string                    `json:"workspaceId"`
	WorkspaceName   string                    `json:"workspaceName"`
	WorkspaceKind   string                    `json:"workspaceKind"`
	BusinessDate    string                    `json:"businessDate"`
	FileName        string                    `json:"fileName"`
	SourceKind      string                    `json:"sourceKind"`
	Status          string                    `json:"status"`
	AIUsed          bool                      `json:"aiUsed"`
	DataCoveragePct int                       `json:"dataCoveragePct"`
	Summary         docintake.SimplifiedSummary `json:"summary"`
	CreatedAt       string                    `json:"createdAt"`
}

type View struct {
	GeneratedAt     string              `json:"generatedAt"`
	Date            string              `json:"date"`
	WorkspaceCount  int                 `json:"workspaceCount"`
	Completeness    CompletenessSummary `json:"completeness"`
	Hotels          HotelsSummary       `json:"hotels"`
	Cafes           CafeSummary         `json:"cafes"`
	RecentDocuments []RecentDocument    `json:"recentDocuments"`
}

func (s *Service) Get(ctx context.Context, p ctxkey.Principal, date string) (View, error) {
	uid, err := id.Parse(p.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	if date != "" {
		if err := hotel.ValidateBusinessDate(date); err != nil {
			return View{}, err
		}
	}
	memberships, err := s.repo.ListMembershipsByUser(ctx, uid)
	if err != nil {
		return View{}, err
	}
	now := s.clock.Now().UTC()
	out := View{
		GeneratedAt:     now.Format(time.RFC3339),
		Date:            date,
		Completeness:    CompletenessSummary{ByWorkspace: []WorkspaceCompleteness{}},
		Hotels:          HotelsSummary{TotalsByCurrency: []CurrencyTotal{}, Hotels: []HotelCard{}},
		Cafes:           CafeSummary{Cafes: []CafeCard{}},
		RecentDocuments: []RecentDocument{},
	}

	currencyMap := map[string]*CurrencyTotal{}
	var occAvail, occUsed int

	for _, m := range memberships {
		if !m.IsActive {
			continue
		}
		ws, err := s.repo.GetWorkspace(ctx, m.WorkspaceID)
		if err != nil || !ws.IsActive {
			continue
		}
		cat, err := s.registry.Catalog(ws.Kind)
		if err != nil {
			continue
		}
		scope := staffscope.Resolve(staffscope.FromMembership(m), cat)
		if !rbac.Allows(scope, rbac.ViewDashboard) {
			continue
		}
		seeMoney := rbac.Allows(scope, rbac.ViewFinance)
		bizDate := date
		if bizDate == "" {
			bizDate = finance.BusinessDate(now, ws.Timezone)
		}
		if out.Date == "" {
			out.Date = bizDate
		}
		out.WorkspaceCount++

		tasks, _ := s.repo.ListTasksByWorkspace(ctx, ws.ID)
		reports, _ := s.repo.ListReports(ctx, ws.ID)
		openTasks, overdueTasks, openReports := countWork(tasks, reports, now)
		docs, _ := s.repo.ListDocuments(ctx, ws.ID, 5)
		for _, d := range docs {
			out.RecentDocuments = append(out.RecentDocuments, RecentDocument{
				ID: d.ID.String(), WorkspaceID: ws.ID.String(), WorkspaceName: ws.Name, WorkspaceKind: string(ws.Kind),
				BusinessDate: d.BusinessDate, FileName: d.FileName, SourceKind: string(d.SourceKind),
				Status: string(d.Status), AIUsed: d.AIUsed, DataCoveragePct: d.DataCoveragePct,
				Summary: d.Summary, CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
			})
		}

		switch ws.Kind {
		case tenancy.KindHotel:
			card, comp := s.collectHotel(ctx, ws, bizDate, now, seeMoney, openTasks, overdueTasks, openReports, len(docs) > 0)
			out.Completeness.ByWorkspace = append(out.Completeness.ByWorkspace, comp)
			out.Hotels.Count++
			out.Hotels.Hotels = append(out.Hotels.Hotels, card)
			if card.OccupancyPct != nil {
				out.Hotels.ConnectedCount++
				out.Hotels.TotalRooms += card.TotalRooms
				out.Hotels.AvailableRooms += card.AvailableRooms
				out.Hotels.OccupiedRooms += card.OccupiedRooms
				out.Hotels.OutOfOrderRooms += card.OutOfOrderRooms
				out.Hotels.TotalArrivals += card.ArrivalsCount
				out.Hotels.TotalDepartures += card.DeparturesCount
				occAvail += card.AvailableRooms
				occUsed += card.OccupiedRooms
			}
			if seeMoney && card.Currency != "" && card.Freshness != string(hotel.FreshUnavailable) {
				ct := currencyMap[card.Currency]
				if ct == nil {
					ct = &CurrencyTotal{Currency: card.Currency}
					currencyMap[card.Currency] = ct
				}
				ct.HotelCount++
				ct.TodayTotalRevenueMinor += card.TotalRevenueMinor
				ct.TodayRoomRevenueMinor += card.RoomRevenueMinor
				if card.HasYesterday {
					ct.HasYesterday = true
					ct.YesterdayRevenueMinor += card.YesterdayRevenueMinor
				}
			}
		case tenancy.KindCafe:
			card, comp := s.collectCafe(ctx, ws, bizDate, seeMoney, openTasks, overdueTasks, openReports)
			out.Completeness.ByWorkspace = append(out.Completeness.ByWorkspace, comp)
			out.Cafes.Count++
			out.Cafes.Cafes = append(out.Cafes.Cafes, card)
		}
	}

	if out.WorkspaceCount == 0 {
		return View{}, errx.Forbidden("forbidden", "no authorized executive workspaces")
	}

	if occAvail > 0 {
		avg := round1(float64(occUsed*100) / float64(occAvail))
		out.Hotels.AvgOccupancyPct = &avg
	}
	for _, ct := range currencyMap {
		if ct.HasYesterday {
			ct.DeltaMinor = ct.TodayTotalRevenueMinor - ct.YesterdayRevenueMinor
			if ct.YesterdayRevenueMinor > 0 {
				pct := round1(float64(ct.DeltaMinor*100) / float64(ct.YesterdayRevenueMinor))
				ct.DeltaPct = &pct
			}
		}
		out.Hotels.TotalsByCurrency = append(out.Hotels.TotalsByCurrency, *ct)
	}
	sort.Slice(out.Hotels.TotalsByCurrency, func(i, j int) bool {
		return out.Hotels.TotalsByCurrency[i].Currency < out.Hotels.TotalsByCurrency[j].Currency
	})
	sort.Slice(out.RecentDocuments, func(i, j int) bool {
		return out.RecentDocuments[i].CreatedAt > out.RecentDocuments[j].CreatedAt
	})
	if len(out.RecentDocuments) > 12 {
		out.RecentDocuments = out.RecentDocuments[:12]
	}

	if len(out.Completeness.ByWorkspace) > 0 {
		sumPct := 0
		for _, w := range out.Completeness.ByWorkspace {
			sumPct += w.Pct
		}
		out.Completeness.OverallPct = sumPct / len(out.Completeness.ByWorkspace)
	}

	return out, nil
}

func (s *Service) collectHotel(ctx context.Context, ws tenancy.Workspace, bizDate string, now time.Time, seeMoney bool, openTasks, overdueTasks, openReports int, hasDocs bool) (HotelCard, WorkspaceCompleteness) {
	cfg, _ := s.repo.GetOHIPConfig(ctx, ws.ID)
	todaySnap, todayErr := s.repo.GetPMSSnapshot(ctx, ws.ID, bizDate)
	hasToday := todayErr == nil
	var snapPtr *hotel.PMSSnapshot
	if hasToday {
		snapPtr = &todaySnap
	}
	fresh := hotel.ComputeFreshness(now, snapPtr, cfg.LastSyncStatus)
	_, closeErr := s.repo.GetCloseByDate(ctx, ws.ID, bizDate)
	hasClose := closeErr == nil

	channels := []ChannelStatus{
		{Key: "pms_snapshot", Entered: hasToday},
		{Key: "reservations", Entered: hasToday && todaySnap.ReservationsCount > 0},
		{Key: "readiness", Entered: hasToday && len(todaySnap.Readiness) > 0},
		{Key: "cash_close", Entered: hasClose},
		{Key: "documents", Entered: hasDocs},
	}
	comp := buildCompleteness(ws, channels)

	card := HotelCard{
		WorkspaceID:        ws.ID.String(),
		Name:               ws.Name,
		Slug:               ws.Slug,
		BusinessDate:       bizDate,
		Freshness:          string(fresh),
		CompletenessPct:    comp.Pct,
		OpenTasks:          openTasks,
		OverdueTasks:       overdueTasks,
		OpenReports:        openReports,
		OHIPConfigured:     cfg.HostURL != "" && cfg.HotelID != "",
		OHOPLastSyncStatus: nonEmpty(string(cfg.LastSyncStatus), string(hotel.SyncNever)),
	}
	if !hasToday {
		return card, comp
	}
	occ := round1(float64(todaySnap.OccupancyBasisPoints()) / 100.0)
	card.Source = string(todaySnap.Source)
	card.TotalRooms = todaySnap.TotalRooms
	card.AvailableRooms = todaySnap.AvailableRooms()
	card.OccupiedRooms = todaySnap.OccupiedRooms
	card.OutOfOrderRooms = todaySnap.OutOfOrderRooms
	card.OccupancyPct = &occ
	card.ArrivalsCount = todaySnap.ArrivalsCount
	card.DeparturesCount = todaySnap.DeparturesCount
	card.ReservationsCount = todaySnap.ReservationsCount
	card.ReadinessPct = todaySnap.ReadinessPct()
	if seeMoney {
		card.Currency = todaySnap.Currency
		card.RoomRevenueMinor = todaySnap.RoomRevenueMinor
		card.FBRevenueMinor = todaySnap.FBRevenueMinor
		card.OtherRevenueMinor = todaySnap.OtherRevenueMinor
		card.TotalRevenueMinor = todaySnap.TotalRevenueMinor()
		card.ADRMinor = todaySnap.ADRMinor()
		card.RevPARMinor = todaySnap.RevPARMinor()

		yday := previousDate(bizDate)
		if ySnap, err := s.repo.GetPMSSnapshot(ctx, ws.ID, yday); err == nil && ySnap.Currency == todaySnap.Currency {
			card.HasYesterday = true
			card.YesterdayRevenueMinor = ySnap.TotalRevenueMinor()
			card.RevenueDeltaMinor = card.TotalRevenueMinor - card.YesterdayRevenueMinor
			if card.YesterdayRevenueMinor > 0 {
				pct := round1(float64(card.RevenueDeltaMinor*100) / float64(card.YesterdayRevenueMinor))
				card.RevenueDeltaPct = &pct
			}
		}
	}
	return card, comp
}

func (s *Service) collectCafe(ctx context.Context, ws tenancy.Workspace, bizDate string, seeMoney bool, openTasks, overdueTasks, openReports int) (CafeCard, WorkspaceCompleteness) {
	tickets, _ := s.repo.ListPOSTickets(ctx, ws.ID, bizDate)
	posClose, posCloseErr := s.repo.GetPOSClose(ctx, ws.ID, bizDate)
	daySales := cafe.SummarizeTickets(bizDate, tickets)
	if posCloseErr == nil && posClose.ID != id.Zero() {
		daySales = posClose.DaySales()
	}

	wasteRows, _ := s.repo.ListWasteTickets(ctx, ws.ID, bizDate)
	dayWaste := cafe.SummarizeWaste(bizDate, wasteRows)

	stockBook, stockErr := s.repo.GetStockCatalog(ctx, ws.ID)
	dayStock := cafe.SummarizeStock(bizDate, stockBook)
	hasStock := stockErr == nil && dayStock.Live()

	_, cashErr := s.repo.GetCloseByDate(ctx, ws.ID, bizDate)
	closeRow, hasCash := finance.Close{}, cashErr == nil
	if hasCash {
		closeRow, _ = s.repo.GetCloseByDate(ctx, ws.ID, bizDate)
	}

	openCheck, openCheckErr := s.repo.GetChecklistRun(ctx, ws.ID, bizDate, cafe.CheckOpen)
	hasCheck := openCheckErr == nil && len(openCheck.Marks) > 0
	handover, hoErr := s.repo.GetOpenHandover(ctx, ws.ID)
	hasHandover := hoErr == nil && handover.ID != id.Zero()
	_, intakeErr := s.repo.GetIntake(ctx, ws.ID, bizDate)
	hasIntake := intakeErr == nil

	recoveries, _ := s.repo.ListRecoveries(ctx, ws.ID)
	openRecoveries := 0
	for _, r := range recoveries {
		if r.Status != cafe.RecoveryClosed {
			openRecoveries++
		}
	}

	channels := []ChannelStatus{
		{Key: "pos", Entered: daySales.Live()},
		{Key: "cash_close", Entered: hasCash},
		{Key: "checklist", Entered: hasCheck},
		{Key: "handover", Entered: hasHandover},
		{Key: "intake", Entered: hasIntake},
		{Key: "stock", Entered: hasStock},
	}
	comp := buildCompleteness(ws, channels)

	card := CafeCard{
		WorkspaceID:     ws.ID.String(),
		Name:            ws.Name,
		Slug:            ws.Slug,
		BusinessDate:    bizDate,
		SalesStatus:     statusWord(daySales.Live()),
		WasteStatus:     statusWord(dayWaste.Live()),
		StockStatus:     statusWord(hasStock),
		CashStatus:      statusWord(hasCash),
		CompletenessPct: comp.Pct,
		OpenTasks:       openTasks,
		OverdueTasks:    overdueTasks,
		OpenReports:     openReports,
		OpenRecoveries:  openRecoveries,
		Profitability:   CafeProfitability{Status: "unavailable", Currency: "IQD"},
	}
	if hasStock {
		card.StockItems = dayStock.ItemCount
		card.StockLowCount = dayStock.LowCount
	}
	if seeMoney {
		if daySales.Live() {
			card.SalesNet = daySales.Net
			card.SalesGross = daySales.Gross
			card.TicketCount = daySales.TicketCount
			card.POSClosed = daySales.Closed
		}
		if dayWaste.Live() {
			card.WasteCost = dayWaste.Cost
			card.WasteGrams = dayWaste.Grams
		}
		if hasCash {
			card.CashVariance = closeRow.Variance
		}
		approvals, _ := s.repo.ListApprovals(ctx, ws.ID)
		var approvedExpenses int64
		pendingCount := 0
		for _, a := range approvals {
			if a.Pending() {
				pendingCount++
			}
			if a.Status == finance.StatusApproved {
				approvedExpenses += a.Amount
			}
		}
		if daySales.Live() {
			est := daySales.Net - dayWaste.Cost - approvedExpenses
			complete := dayWaste.Live() || approvedExpenses > 0
			status := "provisional"
			if complete {
				status = "live"
			}
			var marginPct *float64
			if daySales.Net > 0 {
				m := round1(float64(est*100) / float64(daySales.Net))
				marginPct = &m
			}
			card.Profitability = CafeProfitability{
				Status:                status,
				Currency:              "IQD",
				SalesNet:              daySales.Net,
				WasteCost:             dayWaste.Cost,
				ApprovedExpenses:      approvedExpenses,
				PendingApprovalsCount: pendingCount,
				EstimatedNet:          est,
				MarginPct:             marginPct,
				Complete:              complete,
			}
		} else {
			card.Profitability.PendingApprovalsCount = pendingCount
		}
	}
	return card, comp
}

func buildCompleteness(ws tenancy.Workspace, channels []ChannelStatus) WorkspaceCompleteness {
	entered := 0
	for _, c := range channels {
		if c.Entered {
			entered++
		}
	}
	pct := 0
	if len(channels) > 0 {
		pct = (entered * 100) / len(channels)
	}
	return WorkspaceCompleteness{
		WorkspaceID:   ws.ID.String(),
		WorkspaceName: ws.Name,
		Kind:          string(ws.Kind),
		EnteredCount:  entered,
		ExpectedCount: len(channels),
		Pct:           pct,
		Channels:      channels,
	}
}

func countWork(tasks []work.Task, reports []opsreport.Report, now time.Time) (openTasks, overdueTasks, openReports int) {
	for _, t := range tasks {
		if t.Status == work.StatusCompleted || t.Status == work.StatusCancelled {
			continue
		}
		openTasks++
		if t.Overdue(now) {
			overdueTasks++
		}
	}
	for _, r := range reports {
		if r.Open() {
			openReports++
		}
	}
	return openTasks, overdueTasks, openReports
}

func previousDate(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}

func statusWord(live bool) string {
	if live {
		return "live"
	}
	return "unavailable"
}

func nonEmpty(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
