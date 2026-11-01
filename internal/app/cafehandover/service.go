package cafehandover

import (
	"context"
	"errors"
	"strings"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const listCap = 30

type Service struct {
	defs  DefinitionStore
	store Repository
	work  OpenWork
	users UserReader
	clock clock.Clock
}

func New(defs DefinitionStore, store Repository, work OpenWork, users UserReader, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{defs: defs, store: store, work: work, users: users, clock: clk}
}

type Draft struct {
	BusinessDate string
	FromShift    string
	ToShift      string
	Note         string
}

type RefView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type View struct {
	ID             string    `json:"id,omitempty"`
	BusinessDate   string    `json:"businessDate"`
	Timezone       string    `json:"timezone"`
	FromShift      string    `json:"fromShift,omitempty"`
	ToShift        string    `json:"toShift,omitempty"`
	Status         string    `json:"status,omitempty"`
	Note           string    `json:"note"`
	Shifts         []string  `json:"shifts"`
	Tasks          []RefView `json:"tasks"`
	Recoveries     []RefView `json:"recoveries"`
	Breakdowns     []RefView `json:"breakdowns"`
	OpenedBy       string    `json:"openedBy,omitempty"`
	OpenedByName   string    `json:"openedByName,omitempty"`
	AcceptedBy     string    `json:"acceptedBy,omitempty"`
	AcceptedByName string    `json:"acceptedByName,omitempty"`
	Persisted      bool      `json:"persisted"`
	CanWrite       bool      `json:"canWrite"`
	OpenedAt       string    `json:"openedAt,omitempty"`
	AcceptedAt     string    `json:"acceptedAt,omitempty"`
	UpdatedAt      string    `json:"updatedAt,omitempty"`
}

func (s *Service) Get(ctx context.Context, ac actor.Context) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ViewCafe) {
		return View{}, errx.Forbidden("forbidden", "you cannot view the handover")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	canWrite := rbac.Require(ac.Scope, rbac.ManageReviews)
	open, err := s.store.GetOpenHandover(ctx, ac.Workspace.ID)
	if err == nil {
		return s.toView(ctx, open, def, canWrite, true), nil
	}
	if !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	return s.preview(ctx, ac, def, canWrite)
}

func (s *Service) List(ctx context.Context, ac actor.Context) ([]View, error) {
	if err := requireCafe(ac); err != nil {
		return nil, err
	}
	if !rbac.Require(ac.Scope, rbac.ViewCafe) {
		return nil, errx.Forbidden("forbidden", "you cannot view the handover")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return nil, err
	}
	list, err := s.store.ListHandovers(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	if len(list) > listCap {
		list = list[:listCap]
	}
	canWrite := rbac.Require(ac.Scope, rbac.ManageReviews)
	out := make([]View, 0, len(list))
	for _, rec := range list {
		out = append(out, s.toView(ctx, rec, def, canWrite, true))
	}
	return out, nil
}

func (s *Service) Open(ctx context.Context, ac actor.Context, in Draft) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageReviews) {
		return View{}, errx.Forbidden("forbidden", "only a general manager or floor can open a handover")
	}
	opener, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day, err := s.day(def, in.BusinessDate)
	if err != nil {
		return View{}, err
	}
	if _, err := s.store.GetOpenHandover(ctx, ac.Workspace.ID); err == nil {
		return View{}, errx.Conflict("handover_open", "accept the open handover first")
	} else if !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	tasks, recoveries, breakdowns, err := s.work.SnapshotOpenWork(ctx, ac.Workspace.ID)
	if err != nil {
		return View{}, err
	}
	rec, err := cafe.OpenHandover(ac.Workspace.ID, opener, day, in.FromShift, in.ToShift, in.Note, cafe.RecordedShiftNames(def.Shifts), tasks, recoveries, breakdowns, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.SaveHandover(ctx, rec); err != nil {
		return View{}, err
	}
	return s.toView(ctx, rec, def, true, true), nil
}

func (s *Service) Accept(ctx context.Context, ac actor.Context, handoverID string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageReviews) {
		return View{}, errx.Forbidden("forbidden", "only a general manager or floor can accept a handover")
	}
	incoming, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	hid, err := id.Parse(handoverID)
	if err != nil {
		return View{}, errx.Invalid("invalid_handover", "handover is required")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	rec, err := s.store.GetHandover(ctx, hid)
	if err != nil {
		return View{}, err
	}
	if rec.WorkspaceID != ac.Workspace.ID {
		return View{}, errx.NotFound("handover_not_found", "handover not found")
	}
	if err := rec.Accept(incoming, s.clock.Now().UTC()); err != nil {
		return View{}, err
	}
	if err := s.store.SaveHandover(ctx, rec); err != nil {
		return View{}, err
	}
	return s.toView(ctx, rec, def, true, true), nil
}

func (s *Service) preview(ctx context.Context, ac actor.Context, def cafe.Definition, canWrite bool) (View, error) {
	day := finance.BusinessDate(s.clock.Now(), def.Timezone)
	tasks, recoveries, breakdowns, err := s.work.SnapshotOpenWork(ctx, ac.Workspace.ID)
	if err != nil {
		return View{}, err
	}
	return View{
		BusinessDate: day,
		Timezone:     def.Timezone,
		Shifts:       recorded(def),
		Tasks:        toRefs(tasks),
		Recoveries:   toRefs(recoveries),
		Breakdowns:   toRefs(breakdowns),
		CanWrite:     canWrite,
	}, nil
}

func (s *Service) day(def cafe.Definition, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return finance.BusinessDate(s.clock.Now(), def.Timezone), nil
	}
	return cafe.ParseBusinessDate(raw)
}

func (s *Service) definition(ctx context.Context, ac actor.Context) (cafe.Definition, error) {
	def, err := s.defs.GetCafeDefinition(ctx, ac.Workspace.ID)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return cafe.Seed(ac.Workspace.ID, ac.Workspace.Name, ac.Workspace.Timezone), nil
		}
		return cafe.Definition{}, err
	}
	return def, nil
}

func (s *Service) toView(ctx context.Context, rec cafe.Handover, def cafe.Definition, canWrite, persisted bool) View {
	v := View{
		ID:           rec.ID.String(),
		BusinessDate: rec.BusinessDate,
		Timezone:     def.Timezone,
		FromShift:    rec.FromShift,
		ToShift:      rec.ToShift,
		Status:       rec.Status,
		Note:         rec.Note,
		Shifts:       recorded(def),
		Tasks:        toRefs(rec.Tasks),
		Recoveries:   toRefs(rec.Recoveries),
		Breakdowns:   toRefs(rec.Breakdowns),
		OpenedBy:     rec.OpenedBy.String(),
		Persisted:    persisted,
		CanWrite:     canWrite,
		OpenedAt:     rec.OpenedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    rec.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if rec.AcceptedBy != id.Zero() {
		v.AcceptedBy = rec.AcceptedBy.String()
	}
	if !rec.AcceptedAt.IsZero() {
		v.AcceptedAt = rec.AcceptedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if u, err := s.users.GetUser(ctx, rec.OpenedBy); err == nil {
		v.OpenedByName = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}
	if rec.AcceptedBy != id.Zero() {
		if u, err := s.users.GetUser(ctx, rec.AcceptedBy); err == nil {
			v.AcceptedByName = strings.TrimSpace(u.FirstName + " " + u.LastName)
		}
	}
	return v
}

func recorded(def cafe.Definition) []string {
	shifts := cafe.RecordedShiftNames(def.Shifts)
	if shifts == nil {
		return []string{}
	}
	return shifts
}

func toRefs(in []cafe.HandoverRef) []RefView {
	out := make([]RefView, 0, len(in))
	for _, r := range in {
		out = append(out, RefView{ID: r.ID.String(), Title: r.Title})
	}
	return out
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "handover is only available on a cafe workspace")
	}
	return nil
}
