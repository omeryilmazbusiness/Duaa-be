package cafecheck

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

type DraftItem struct {
	ID     string
	Kind   string
	Title  string
	Active *bool
}

type LineView struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Sort       int    `json:"sort"`
	Active     bool   `json:"active"`
	Done       bool   `json:"done"`
	DoneAt     string `json:"doneAt,omitempty"`
	DoneBy     string `json:"doneBy,omitempty"`
	DoneByName string `json:"doneByName,omitempty"`
}

type KindPulse struct {
	Total     int `json:"total"`
	Done      int `json:"done"`
	Remaining int `json:"remaining"`
}

type View struct {
	BusinessDate string    `json:"businessDate"`
	Timezone     string    `json:"timezone"`
	Items        []LineView `json:"items"`
	Open         KindPulse `json:"open"`
	Close        KindPulse `json:"close"`
	HasList      bool      `json:"hasList"`
	CanEdit      bool      `json:"canEdit"`
	CanCheck     bool      `json:"canCheck"`
}

func (s *Service) Get(ctx context.Context, ac actor.Context, date string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ViewCafe) {
		return View{}, errx.Forbidden("forbidden", "you cannot view the opening list")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	tpl, err := s.template(ctx, ac.Workspace.ID)
	if err != nil {
		return View{}, err
	}
	return s.toView(ctx, tpl, day, def, ac)
}

func (s *Service) SaveTemplate(ctx context.Context, ac actor.Context, in []DraftItem) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageWorkspace) {
		return View{}, errx.Forbidden("forbidden", "only a general manager can write the opening list")
	}
	actorID, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	items := make([]cafe.CheckItem, 0, len(in))
	for _, row := range in {
		item := cafe.CheckItem{Kind: cafe.CheckKind(row.Kind), Title: row.Title, Active: true}
		if row.Active != nil {
			item.Active = *row.Active
		}
		if row.ID != "" {
			parsed, err := id.Parse(row.ID)
			if err != nil {
				return View{}, errx.Invalid("invalid_check", "checklist line is not recorded")
			}
			item.ID = parsed
		}
		items = append(items, item)
	}
	rec, err := cafe.ReplaceTemplate(ac.Workspace.ID, actorID, items, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.SaveChecklistTemplate(ctx, rec); err != nil {
		return View{}, err
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day := finance.BusinessDate(s.clock.Now(), def.Timezone)
	return s.toView(ctx, rec, day, def, ac)
}

func (s *Service) Toggle(ctx context.Context, ac actor.Context, date, itemID string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageReviews) {
		return View{}, errx.Forbidden("forbidden", "only a general manager or floor can tick the list")
	}
	actorID, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	iid, err := id.Parse(itemID)
	if err != nil {
		return View{}, errx.Invalid("unknown_check", "checklist line is not recorded")
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day, err := s.day(def, date)
	if err != nil {
		return View{}, err
	}
	tpl, err := s.template(ctx, ac.Workspace.ID)
	if err != nil {
		return View{}, err
	}
	item, ok := tpl.Item(iid)
	if !ok {
		return View{}, errx.Invalid("unknown_check", "checklist line is not recorded")
	}
	var cur *cafe.ChecklistRun
	existing, err := s.store.GetChecklistRun(ctx, ac.Workspace.ID, day, item.Kind)
	if err == nil {
		cur = &existing
	} else if !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	rec, err := cafe.ToggleRun(cur, ac.Workspace.ID, actorID, day, item, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.SaveChecklistRun(ctx, rec); err != nil {
		return View{}, err
	}
	return s.toView(ctx, tpl, day, def, ac)
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

func (s *Service) template(ctx context.Context, workspaceID id.ID) (cafe.ChecklistTemplate, error) {
	tpl, err := s.store.GetChecklistTemplate(ctx, workspaceID)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return cafe.ChecklistTemplate{}, err
	}
	if errors.Is(err, errx.ErrNotFound) {
		return cafe.ChecklistTemplate{WorkspaceID: workspaceID, Items: []cafe.CheckItem{}}, nil
	}
	return tpl, nil
}

func (s *Service) toView(ctx context.Context, tpl cafe.ChecklistTemplate, day string, def cafe.Definition, ac actor.Context) (View, error) {
	openRun, _ := s.store.GetChecklistRun(ctx, ac.Workspace.ID, day, cafe.CheckOpen)
	closeRun, _ := s.store.GetChecklistRun(ctx, ac.Workspace.ID, day, cafe.CheckClose)
	openItems := tpl.ActiveItems(cafe.CheckOpen)
	closeItems := tpl.ActiveItems(cafe.CheckClose)
	items := make([]LineView, 0, len(tpl.Items))
	for _, item := range tpl.Items {
		run := openRun
		if item.Kind == cafe.CheckClose {
			run = closeRun
		}
		line := LineView{
			ID: item.ID.String(), Kind: string(item.Kind), Title: item.Title, Sort: item.Sort, Active: item.Active,
		}
		for _, m := range run.Marks {
			if m.ItemID != item.ID {
				continue
			}
			line.Done = true
			line.DoneAt = m.DoneAt.UTC().Format("2006-01-02T15:04:05Z07:00")
			line.DoneBy = m.DoneBy.String()
			if u, err := s.users.GetUser(ctx, m.DoneBy); err == nil {
				line.DoneByName = strings.TrimSpace(u.FirstName + " " + u.LastName)
			}
			break
		}
		items = append(items, line)
	}
	if items == nil {
		items = []LineView{}
	}
	return View{
		BusinessDate: day,
		Timezone:     def.Timezone,
		Items:        items,
		Open:         KindPulse{Total: len(openItems), Done: openRun.DoneCount(openItems), Remaining: openRun.Remaining(openItems)},
		Close:        KindPulse{Total: len(closeItems), Done: closeRun.DoneCount(closeItems), Remaining: closeRun.Remaining(closeItems)},
		HasList:      len(openItems)+len(closeItems) > 0,
		CanEdit:      rbac.Require(ac.Scope, rbac.ManageWorkspace),
		CanCheck:     rbac.Require(ac.Scope, rbac.ManageReviews),
	}, nil
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "opening list is only available on a cafe workspace")
	}
	return nil
}
