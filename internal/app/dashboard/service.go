package dashboard

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	tasks     TaskReader
	members   MembershipReader
	reports   ReportReader
	users     UserReader
	cash      CashReader
	approvals ApprovalReader
	sales     SalesReader
	clock     clock.Clock
}

func New(tasks TaskReader, members MembershipReader, reports ReportReader, users UserReader, cash CashReader, approvals ApprovalReader, sales SalesReader, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{tasks: tasks, members: members, reports: reports, users: users, cash: cash, approvals: approvals, sales: sales, clock: clk}
}

type TileView struct {
	Module    string `json:"module"`
	Status    string `json:"status"`
	ReasonKey string `json:"reasonKey,omitempty"`
	Value     int64  `json:"value"`
	LabelKey  string `json:"labelKey"`
}

type PulseView struct {
	Employees        int `json:"employees"`
	OpenTasks        int `json:"openTasks"`
	OverdueTasks     int `json:"overdueTasks"`
	CompletedToday   int `json:"completedToday"`
	OpenReports      int `json:"openReports"`
	OverloadedStaff  int `json:"overloadedStaff"`
	PendingApprovals int `json:"pendingApprovals"`
}

type LoadView struct {
	UserID       string `json:"userId"`
	Name         string `json:"name"`
	Department   string `json:"department,omitempty"`
	OpenTasks    int    `json:"openTasks"`
	OverdueTasks int    `json:"overdueTasks"`
}

type CashView struct {
	Status         string `json:"status"`
	ReasonKey      string `json:"reasonKey,omitempty"`
	BusinessDate   string `json:"businessDate"`
	CountedAmount  *int64 `json:"countedAmount,omitempty"`
	ExpectedAmount *int64 `json:"expectedAmount,omitempty"`
	Variance       int64  `json:"variance"`
	ClosedAt       string `json:"closedAt,omitempty"`
}

type SalesView struct {
	Status       string `json:"status"`
	ReasonKey    string `json:"reasonKey,omitempty"`
	BusinessDate string `json:"businessDate"`
	TicketCount  int    `json:"ticketCount"`
	SaleCount    int    `json:"saleCount"`
	ReturnCount  int    `json:"returnCount"`
	Gross        int64  `json:"gross"`
	Discount     int64  `json:"discount"`
	Returns      int64  `json:"returns"`
	Net          int64  `json:"net"`
	Basket       int64  `json:"basket"`
	Cash         int64  `json:"cash"`
	Card         int64  `json:"card"`
	Other        int64  `json:"other"`
	Closed       bool   `json:"closed"`
	ClosedAt     string `json:"closedAt,omitempty"`
}

type ApprovalView struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Supplier string `json:"supplier"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Evidence string `json:"evidence,omitempty"`
	Version  int    `json:"version"`
	Status   string `json:"status"`
}

type View struct {
	GeneratedAt string         `json:"generatedAt"`
	Date        string         `json:"date"`
	Kind        string         `json:"workspaceKind"`
	Pulse       PulseView      `json:"pulse"`
	Tiles       []TileView     `json:"tiles"`
	Cash        *CashView      `json:"cash,omitempty"`
	Sales       *SalesView     `json:"sales,omitempty"`
	Approvals   []ApprovalView `json:"approvals,omitempty"`
	Workload    []LoadView     `json:"workload"`
}

func (s *Service) ForGM(ctx context.Context, ac actor.Context) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.ViewDashboard) {
		return View{}, errx.Forbidden("forbidden", "cannot view dashboard")
	}
	now := s.clock.Now()
	date := finance.BusinessDate(now, ac.Workspace.Timezone)
	seeMoney := rbac.Allows(ac.Scope, rbac.ViewFinance)

	var (
		tasks     []work.Task
		members   []identity.Membership
		reports   []opsreport.Report
		closeRow  finance.Close
		closeErr  error
		approvals []finance.Approval
		tickets   []cafe.Ticket
		posClose  cafe.POSClose
		posErr    error
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		list, err := s.tasks.ListTasksByWorkspace(gctx, ac.Workspace.ID)
		tasks = list
		return err
	})
	g.Go(func() error {
		list, err := s.members.ListWorkspaceMemberships(gctx, ac.Workspace.ID)
		members = list
		return err
	})
	if s.reports != nil {
		g.Go(func() error {
			list, err := s.reports.ListReports(gctx, ac.Workspace.ID)
			reports = list
			return err
		})
	}
	if seeMoney && s.cash != nil {
		g.Go(func() error {
			row, err := s.cash.GetCloseByDate(gctx, ac.Workspace.ID, date)
			closeRow, closeErr = row, err
			if errors.Is(err, errx.ErrNotFound) {
				return nil
			}
			return err
		})
	}
	if seeMoney && s.approvals != nil {
		g.Go(func() error {
			list, err := s.approvals.ListApprovals(gctx, ac.Workspace.ID)
			approvals = list
			return err
		})
	}
	if seeMoney && s.sales != nil {
		g.Go(func() error {
			list, err := s.sales.ListPOSTickets(gctx, ac.Workspace.ID, date)
			tickets = list
			return err
		})
		g.Go(func() error {
			row, err := s.sales.GetPOSClose(gctx, ac.Workspace.ID, date)
			posClose, posErr = row, err
			if errors.Is(err, errx.ErrNotFound) {
				return nil
			}
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return View{}, err
	}

	pulse, load := s.pulse(ctx, ac.Workspace.ID, members, tasks, reports, now)
	pending := 0
	packViews := make([]ApprovalView, 0)
	for _, a := range approvals {
		if a.WorkspaceID != ac.Workspace.ID {
			continue
		}
		if a.Pending() {
			pending++
			packViews = append(packViews, ApprovalView{
				ID: a.ID.String(), Title: a.Title, Supplier: a.Supplier,
				Amount: a.Amount, Currency: a.Currency, Evidence: a.Evidence,
				Version: a.Version, Status: a.Status,
			})
		}
	}
	pulse.PendingApprovals = pending
	sort.Slice(packViews, func(i, j int) bool { return packViews[i].Amount > packViews[j].Amount })

	var salesView *SalesView
	if seeMoney {
		salesView = &SalesView{Status: "unavailable", ReasonKey: "dash.posNotWired", BusinessDate: date}
		day := cafe.SummarizeTickets(date, tickets)
		if posErr == nil && posClose.ID != id.Zero() {
			tenders := day
			day = posClose.DaySales()
			day.Cash, day.Card, day.Other = tenders.Cash, tenders.Card, tenders.Other
		}
		if day.Live() {
			salesView = &SalesView{
				Status: "live", BusinessDate: date, TicketCount: day.TicketCount, SaleCount: day.SaleCount,
				ReturnCount: day.ReturnCount, Gross: day.Gross, Discount: day.Discount, Returns: day.Returns,
				Net: day.Net, Basket: day.Basket, Cash: day.Cash, Card: day.Card, Other: day.Other, Closed: day.Closed,
			}
			if day.Closed {
				salesView.ClosedAt = posClose.ClosedAt.UTC().Format(time.RFC3339)
			}
		}
	}

	var cashView *CashView
	if seeMoney {
		cashView = &CashView{Status: "unavailable", ReasonKey: "brief.cashNotClosed", BusinessDate: date}
		if closeErr == nil && closeRow.ID != id.Zero() {
			counted := closeRow.CountedAmount
			cashView = &CashView{
				Status:         "live",
				BusinessDate:   closeRow.BusinessDate,
				CountedAmount:  &counted,
				ExpectedAmount: closeRow.ExpectedAmount,
				Variance:       closeRow.Variance,
				ClosedAt:       closeRow.UpdatedAt.UTC().Format(time.RFC3339),
			}
		}
	}

	tiles := make([]TileView, 0, len(ac.Catalog.DashboardModules()))
	for _, mod := range ac.Catalog.DashboardModules() {
		if moneyModule(mod) && !seeMoney {
			continue
		}
		tiles = append(tiles, s.tile(mod, pulse, cashView, salesView, pending))
	}

	out := View{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Date:        date,
		Kind:        string(ac.Workspace.Kind),
		Pulse:       pulse,
		Tiles:       tiles,
		Cash:        cashView,
		Sales:       salesView,
		Workload:    load,
	}
	if seeMoney {
		out.Approvals = packViews
	}
	return out, nil
}

func moneyModule(mod tenancy.DashboardModule) bool {
	switch mod {
	case tenancy.ModuleCash, tenancy.ModuleApprovals, tenancy.ModuleSales, tenancy.ModuleWaste:
		return true
	default:
		return false
	}
}

func (s *Service) tile(mod tenancy.DashboardModule, pulse PulseView, cash *CashView, sales *SalesView, pending int) TileView {
	switch mod {
	case tenancy.ModulePulse:
		return TileView{Module: string(mod), Status: "live", Value: int64(pulse.OverdueTasks), LabelKey: "overdue"}
	case tenancy.ModuleReports:
		return TileView{Module: string(mod), Status: "live", Value: int64(pulse.OpenReports), LabelKey: "employeeReports"}
	case tenancy.ModuleCash:
		tile := TileView{Module: string(mod), Status: "unavailable", ReasonKey: "brief.cashNotClosed", LabelKey: "sourceCash"}
		if cash != nil && cash.Status == "live" {
			tile.Status = "live"
			tile.ReasonKey = ""
			tile.Value = cash.Variance
		}
		return tile
	case tenancy.ModuleApprovals:
		return TileView{Module: string(mod), Status: "live", Value: int64(pending), LabelKey: "pendingApprovals"}
	case tenancy.ModuleSales:
		tile := TileView{Module: string(mod), Status: "unavailable", ReasonKey: "dash.posNotWired", LabelKey: "sales"}
		if sales != nil && sales.Status == "live" {
			tile.Status = "live"
			tile.ReasonKey = ""
			tile.Value = sales.Net
		}
		return tile
	case tenancy.ModuleWaste:
		return TileView{Module: string(mod), Status: "unavailable", ReasonKey: "dash.wasteNotWired", LabelKey: "waste"}
	case tenancy.ModuleOccupancy:
		return TileView{Module: string(mod), Status: "unavailable", ReasonKey: "dash.occupancyNotWired", LabelKey: "occupancy"}
	default:
		return TileView{Module: string(mod), Status: "unavailable", ReasonKey: "brief.sourceNotWired", LabelKey: string(mod)}
	}
}

func (s *Service) pulse(ctx context.Context, workspaceID id.ID, members []identity.Membership, tasks []work.Task, reports []opsreport.Report, now time.Time) (PulseView, []LoadView) {
	employees := 0
	byUser := map[id.ID]identity.Membership{}
	for _, m := range members {
		if m.WorkspaceID != workspaceID || !m.IsActive {
			continue
		}
		if m.Role == identity.RolePersonnel {
			employees++
			byUser[m.UserID] = m
		}
	}
	type counters struct{ open, overdue int }
	load := map[id.ID]*counters{}
	pulse := PulseView{Employees: employees}
	for _, task := range tasks {
		if task.WorkspaceID != workspaceID {
			continue
		}
		if task.Status == work.StatusCompleted {
			if task.CompletedAt != nil {
				cy, cm, cd := task.CompletedAt.UTC().Date()
				ny, nm, nd := now.UTC().Date()
				if cy == ny && cm == nm && cd == nd {
					pulse.CompletedToday++
				}
			}
			continue
		}
		if task.Status == work.StatusCancelled {
			continue
		}
		pulse.OpenTasks++
		c := load[task.AssigneeUserID]
		if c == nil {
			c = &counters{}
			load[task.AssigneeUserID] = c
		}
		c.open++
		if task.Overdue(now) {
			pulse.OverdueTasks++
			c.overdue++
		}
	}
	for _, r := range reports {
		if r.WorkspaceID == workspaceID && r.Open() {
			pulse.OpenReports++
		}
	}
	out := make([]LoadView, 0)
	for uid, c := range load {
		if c.open < 2 {
			continue
		}
		name, dept := uid.String(), ""
		if m, ok := byUser[uid]; ok {
			dept = m.Department
		}
		if s.users != nil {
			if u, err := s.users.GetUser(ctx, uid); err == nil {
				name = strings.TrimSpace(u.FirstName + " " + u.LastName)
			}
		}
		out = append(out, LoadView{UserID: uid.String(), Name: name, Department: dept, OpenTasks: c.open, OverdueTasks: c.overdue})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTasks > out[j].OpenTasks })
	pulse.OverloadedStaff = len(out)
	return pulse, out
}
