package cafepos

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	defs  DefinitionStore
	store Repository
	clock clock.Clock
}

func New(defs DefinitionStore, store Repository, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{defs: defs, store: store, clock: clk}
}

type TicketView struct {
	ID           string `json:"id"`
	ExternalRef  string `json:"externalRef"`
	BusinessDate string `json:"businessDate"`
	Kind         string `json:"kind"`
	Lines        int    `json:"lines"`
	Gross        int64  `json:"gross"`
	Discount     int64  `json:"discount"`
	Net          int64  `json:"net"`
	Tender       string `json:"tender"`
	RecordedAt   string `json:"recordedAt"`
}

type View struct {
	BusinessDate string       `json:"businessDate"`
	Timezone     string       `json:"timezone"`
	Status       string       `json:"status"`
	ReasonKey    string       `json:"reasonKey,omitempty"`
	TicketCount  int          `json:"ticketCount"`
	SaleCount    int          `json:"saleCount"`
	ReturnCount  int          `json:"returnCount"`
	Gross        int64        `json:"gross"`
	Discount     int64        `json:"discount"`
	Returns      int64        `json:"returns"`
	Net          int64        `json:"net"`
	Basket       int64        `json:"basket"`
	Cash         int64        `json:"cash"`
	Card         int64        `json:"card"`
	Other        int64        `json:"other"`
	Closed       bool         `json:"closed"`
	ClosedAt     string       `json:"closedAt,omitempty"`
	Tickets      []TicketView `json:"tickets"`
	CanRecord    bool         `json:"canRecord"`
}

func (s *Service) Get(ctx context.Context, ac actor.Context, date string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.ViewFinance) {
		return View{}, errx.Forbidden("forbidden", "only the general manager reads POS")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac, def, day)
}

func (s *Service) Record(ctx context.Context, ac actor.Context, date, externalRef, kind, tender string, lines int, gross, discount int64) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.RecordPOS) {
		return View{}, errx.Forbidden("forbidden", "only the general manager records POS tickets")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	if def.POSStatus == cafe.POSNone {
		return View{}, cafe.POSNotUsedError()
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	if _, err := s.store.GetPOSClose(ctx, ac.Workspace.ID, day); err == nil {
		return View{}, cafe.POSDayClosedConflict()
	} else if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	ticket, err := cafe.NewTicket(ac.Workspace.ID, uid, day, externalRef, kind, tender, lines, gross, discount, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.CreatePOSTicket(ctx, ticket); err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac, def, day)
}

func (s *Service) Close(ctx context.Context, ac actor.Context, date string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.RecordPOS) {
		return View{}, errx.Forbidden("forbidden", "only the general manager closes a POS day")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	if def.POSStatus == cafe.POSNone {
		return View{}, cafe.POSNotUsedError()
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	if _, err := s.store.GetPOSClose(ctx, ac.Workspace.ID, day); err == nil {
		return View{}, cafe.POSDayClosedConflict()
	} else if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	tickets, err := s.store.ListPOSTickets(ctx, ac.Workspace.ID, day)
	if err != nil {
		return View{}, err
	}
	closed, err := cafe.ClosePOSDay(ac.Workspace.ID, uid, day, tickets, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.CreatePOSClose(ctx, closed); err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac, def, day)
}

func (s *Service) toView(ctx context.Context, ac actor.Context, def cafe.Definition, day string) (View, error) {
	tickets, err := s.store.ListPOSTickets(ctx, ac.Workspace.ID, day)
	if err != nil {
		return View{}, err
	}
	sort.Slice(tickets, func(i, j int) bool { return tickets[i].RecordedAt.After(tickets[j].RecordedAt) })
	sales := cafe.SummarizeTickets(day, tickets)
	closedAt := ""
	if closed, err := s.store.GetPOSClose(ctx, ac.Workspace.ID, day); err == nil {
		tenders := sales
		sales = closed.DaySales()
		sales.Cash, sales.Card, sales.Other = tenders.Cash, tenders.Card, tenders.Other
		closedAt = closed.ClosedAt.UTC().Format(time.RFC3339)
	} else if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	view := View{
		BusinessDate: day,
		Timezone:     def.Timezone,
		Status:       "unavailable",
		ReasonKey:    "dash.posNotWired",
		Tickets:      make([]TicketView, 0, len(tickets)),
		CanRecord:    rbac.Allows(ac.Scope, rbac.RecordPOS) && def.POSStatus != cafe.POSNone && !sales.Closed,
	}
	if sales.Live() {
		view.Status = "live"
		view.ReasonKey = ""
		view.TicketCount = sales.TicketCount
		view.SaleCount = sales.SaleCount
		view.ReturnCount = sales.ReturnCount
		view.Gross = sales.Gross
		view.Discount = sales.Discount
		view.Returns = sales.Returns
		view.Net = sales.Net
		view.Basket = sales.Basket
		view.Cash = sales.Cash
		view.Card = sales.Card
		view.Other = sales.Other
		view.Closed = sales.Closed
		view.ClosedAt = closedAt
	}
	for _, t := range tickets {
		view.Tickets = append(view.Tickets, TicketView{
			ID: t.ID.String(), ExternalRef: t.ExternalRef, BusinessDate: t.BusinessDate,
			Kind: string(t.Kind), Lines: t.Lines, Gross: t.Gross, Discount: t.Discount, Net: t.Net,
			Tender: string(t.Tender), RecordedAt: t.RecordedAt.UTC().Format(time.RFC3339),
		})
	}
	return view, nil
}

func (s *Service) definition(ctx context.Context, ac actor.Context) (cafe.Definition, error) {
	def, err := s.defs.GetCafeDefinition(ctx, ac.Workspace.ID)
	if err == nil {
		return def, nil
	}
	if errors.Is(err, errx.ErrNotFound) {
		return cafe.Seed(ac.Workspace.ID, ac.Workspace.Name, ac.Workspace.Timezone), nil
	}
	return cafe.Definition{}, err
}

func (s *Service) day(def cafe.Definition, date string) (string, error) {
	if date == "" {
		return finance.BusinessDate(s.clock.Now(), def.Timezone), nil
	}
	if !validDay(date) {
		return "", errx.Invalid("invalid_business_date", "business date must be YYYY-MM-DD")
	}
	return date, nil
}

func validDay(raw string) bool {
	_, err := time.Parse("2006-01-02", raw)
	return err == nil && len(raw) == 10
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "POS is only available on a cafe workspace")
	}
	return nil
}
