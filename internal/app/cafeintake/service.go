package cafeintake

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
	users UserReader
	clock clock.Clock
}

func New(defs DefinitionStore, store Repository, users UserReader, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{defs: defs, store: store, users: users, clock: clk}
}

type Draft struct {
	BusinessDate string
	ShiftName    string
	CloseNote    string
	WasteNote    string
	IncidentNote string
}

type View struct {
	ID           string   `json:"id,omitempty"`
	BusinessDate string   `json:"businessDate"`
	Timezone     string   `json:"timezone"`
	ShiftName    string   `json:"shiftName,omitempty"`
	Shifts       []string `json:"shifts"`
	CloseNote    string   `json:"closeNote"`
	WasteNote    string   `json:"wasteNote"`
	IncidentNote string   `json:"incidentNote"`
	AuthorUserID string   `json:"authorUserId,omitempty"`
	AuthorName   string   `json:"authorName,omitempty"`
	Persisted    bool     `json:"persisted"`
	CanWrite     bool     `json:"canWrite"`
	UpdatedAt    string   `json:"updatedAt,omitempty"`
}

func (s *Service) Get(ctx context.Context, ac actor.Context, date string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ViewCafe) {
		return View{}, errx.Forbidden("forbidden", "you cannot view the daily close")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	rec, err := s.store.GetIntake(ctx, ac.Workspace.ID, day)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	if errors.Is(err, errx.ErrNotFound) {
		return emptyView(def, day, rbac.Require(ac.Scope, rbac.ManageReviews)), nil
	}
	return s.toView(ctx, rec, def, rbac.Require(ac.Scope, rbac.ManageReviews)), nil
}

func (s *Service) List(ctx context.Context, ac actor.Context) ([]View, error) {
	if err := requireCafe(ac); err != nil {
		return nil, err
	}
	if !rbac.Require(ac.Scope, rbac.ViewCafe) {
		return nil, errx.Forbidden("forbidden", "you cannot view the daily close")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return nil, err
	}
	list, err := s.store.ListIntakes(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	if len(list) > listCap {
		list = list[:listCap]
	}
	canWrite := rbac.Require(ac.Scope, rbac.ManageReviews)
	out := make([]View, 0, len(list))
	for _, rec := range list {
		out = append(out, s.toView(ctx, rec, def, canWrite))
	}
	return out, nil
}

func (s *Service) Save(ctx context.Context, ac actor.Context, in Draft) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageReviews) {
		return View{}, errx.Forbidden("forbidden", "only a general manager or floor can write the daily close")
	}
	author, err := id.Parse(ac.Principal.UserID)
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
	var cur *cafe.Intake
	existing, err := s.store.GetIntake(ctx, ac.Workspace.ID, day)
	if err == nil {
		cur = &existing
	} else if !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	rec, err := cafe.UpsertIntake(cur, ac.Workspace.ID, author, day, in.ShiftName, in.CloseNote, in.WasteNote, in.IncidentNote, cafe.RecordedShiftNames(def.Shifts), s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.SaveIntake(ctx, rec); err != nil {
		return View{}, err
	}
	return s.toView(ctx, rec, def, true), nil
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

func (s *Service) toView(ctx context.Context, rec cafe.Intake, def cafe.Definition, canWrite bool) View {
	v := View{
		ID:           rec.ID.String(),
		BusinessDate: rec.BusinessDate,
		Timezone:     def.Timezone,
		ShiftName:    rec.ShiftName,
		Shifts:       cafe.RecordedShiftNames(def.Shifts),
		CloseNote:    rec.CloseNote,
		WasteNote:    rec.WasteNote,
		IncidentNote: rec.IncidentNote,
		AuthorUserID: rec.AuthorUserID.String(),
		Persisted:    true,
		CanWrite:     canWrite,
		UpdatedAt:    rec.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if v.Shifts == nil {
		v.Shifts = []string{}
	}
	if u, err := s.users.GetUser(ctx, rec.AuthorUserID); err == nil {
		v.AuthorName = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}
	return v
}

func emptyView(def cafe.Definition, day string, canWrite bool) View {
	shifts := cafe.RecordedShiftNames(def.Shifts)
	if shifts == nil {
		shifts = []string{}
	}
	return View{
		BusinessDate: day,
		Timezone:     def.Timezone,
		Shifts:       shifts,
		CanWrite:     canWrite,
	}
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "daily close is only available on a cafe workspace")
	}
	return nil
}
