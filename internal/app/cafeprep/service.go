package cafeprep

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
	TicketRef    string `json:"ticketRef"`
	TableLabel   string `json:"tableLabel,omitempty"`
	Station      string `json:"station"`
	Status       string `json:"status"`
	WaitMinutes  int    `json:"waitMinutes"`
	OpenMinutes  int    `json:"openMinutes"`
	ReceivedAt   string `json:"receivedAt"`
	ReadyAt      string `json:"readyAt,omitempty"`
	ServedAt     string `json:"servedAt,omitempty"`
	BusinessDate string `json:"businessDate"`
}

type View struct {
	BusinessDate       string       `json:"businessDate"`
	Timezone           string       `json:"timezone"`
	Status             string       `json:"status"`
	ReasonKey          string       `json:"reasonKey,omitempty"`
	TicketCount        int          `json:"ticketCount"`
	WaitingCount       int          `json:"waitingCount"`
	ReadyCount         int          `json:"readyCount"`
	ServedCount        int          `json:"servedCount"`
	LongestOpenMinutes int          `json:"longestOpenMinutes"`
	Tickets            []TicketView `json:"tickets"`
	CanRecord          bool         `json:"canRecord"`
}

func (s *Service) Get(ctx context.Context, ac actor.Context, date string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.ViewCafe) {
		return View{}, errx.Forbidden("forbidden", "cannot view prep times")
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

func (s *Service) Record(ctx context.Context, ac actor.Context, date, station, ticketRef, tableLabel string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.RecordPrep) {
		return View{}, errx.Forbidden("forbidden", "cannot record prep times")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	ticket, err := cafe.NewPrepTicket(ac.Workspace.ID, uid, day, station, ticketRef, tableLabel, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.CreatePrepTicket(ctx, ticket); err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac, def, day)
}

func (s *Service) MarkReady(ctx context.Context, ac actor.Context, ticketID string) (View, error) {
	return s.advance(ctx, ac, ticketID, func(ticket *cafe.PrepTicket, actorID id.ID, now time.Time) error {
		return ticket.MarkReady(actorID, now)
	})
}

func (s *Service) MarkServed(ctx context.Context, ac actor.Context, ticketID string) (View, error) {
	return s.advance(ctx, ac, ticketID, func(ticket *cafe.PrepTicket, actorID id.ID, now time.Time) error {
		return ticket.MarkServed(actorID, now)
	})
}

func (s *Service) advance(ctx context.Context, ac actor.Context, ticketID string, act func(*cafe.PrepTicket, id.ID, time.Time) error) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.RecordPrep) {
		return View{}, errx.Forbidden("forbidden", "cannot record prep times")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	tid, err := id.Parse(ticketID)
	if err != nil {
		return View{}, cafe.PrepNotFoundError()
	}
	ticket, err := s.store.GetPrepTicket(ctx, tid)
	if err != nil {
		return View{}, err
	}
	if ticket.WorkspaceID != ac.Workspace.ID {
		return View{}, cafe.PrepNotFoundError()
	}
	if err := act(&ticket, uid, s.clock.Now().UTC()); err != nil {
		return View{}, err
	}
	if err := s.store.SavePrepTicket(ctx, ticket); err != nil {
		return View{}, err
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac, def, ticket.BusinessDate)
}

func (s *Service) toView(ctx context.Context, ac actor.Context, def cafe.Definition, day string) (View, error) {
	tickets, err := s.store.ListPrepTickets(ctx, ac.Workspace.ID, day)
	if err != nil {
		return View{}, err
	}
	now := s.clock.Now().UTC()
	sort.SliceStable(tickets, func(i, j int) bool {
		oi, oj := tickets[i].Open(), tickets[j].Open()
		if oi != oj {
			return oi
		}
		if tickets[i].Status() != tickets[j].Status() {
			return tickets[i].Status() == cafe.PrepWaiting
		}
		return tickets[i].ReceivedAt.After(tickets[j].ReceivedAt)
	})
	dayPrep := cafe.SummarizePrep(day, tickets, now)
	view := View{
		BusinessDate: day,
		Timezone:     def.Timezone,
		Status:       "unavailable",
		ReasonKey:    "dash.prepNotWired",
		Tickets:      make([]TicketView, 0, len(tickets)),
		CanRecord:    rbac.Allows(ac.Scope, rbac.RecordPrep),
	}
	if dayPrep.Live() {
		view.Status = "live"
		view.ReasonKey = ""
		view.TicketCount = dayPrep.TicketCount
		view.WaitingCount = dayPrep.WaitingCount
		view.ReadyCount = dayPrep.ReadyCount
		view.ServedCount = dayPrep.ServedCount
		view.LongestOpenMinutes = dayPrep.LongestOpenMinutes
	}
	for _, t := range tickets {
		row := TicketView{
			ID: t.ID.String(), TicketRef: t.TicketRef, TableLabel: t.TableLabel, Station: string(t.Station),
			Status: string(t.Status()), WaitMinutes: t.WaitMinutes(now), OpenMinutes: t.OpenMinutes(now),
			ReceivedAt: t.ReceivedAt.UTC().Format(time.RFC3339), BusinessDate: t.BusinessDate,
		}
		if !t.ReadyAt.IsZero() {
			row.ReadyAt = t.ReadyAt.UTC().Format(time.RFC3339)
		}
		if !t.ServedAt.IsZero() {
			row.ServedAt = t.ServedAt.UTC().Format(time.RFC3339)
		}
		view.Tickets = append(view.Tickets, row)
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
	if _, err := time.Parse("2006-01-02", date); err != nil || len(date) != 10 {
		return "", errx.Invalid("invalid_business_date", "business date must be YYYY-MM-DD")
	}
	return date, nil
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "prep times are only available on a cafe workspace")
	}
	return nil
}
