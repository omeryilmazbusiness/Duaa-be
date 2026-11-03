package cafewaste

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
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

type RecipeLineView struct {
	Title string `json:"title"`
	Grams int64  `json:"grams"`
}

type RecipeView struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Station string           `json:"station"`
	Lines   []RecipeLineView `json:"lines"`
}

type TicketView struct {
	ID           string `json:"id"`
	Item         string `json:"item"`
	Station      string `json:"station"`
	Grams        int64  `json:"grams"`
	Cost         int64  `json:"cost"`
	Reason       string `json:"reason"`
	Note         string `json:"note,omitempty"`
	RecipeID     string `json:"recipeId,omitempty"`
	RecordedAt   string `json:"recordedAt"`
	BusinessDate string `json:"businessDate"`
}

type View struct {
	BusinessDate   string       `json:"businessDate"`
	Timezone       string       `json:"timezone"`
	Status         string       `json:"status"`
	ReasonKey      string       `json:"reasonKey,omitempty"`
	TicketCount    int          `json:"ticketCount"`
	Grams          int64        `json:"grams"`
	Cost           int64        `json:"cost"`
	KitchenGrams   int64        `json:"kitchenGrams"`
	BarGrams       int64        `json:"barGrams"`
	Tickets        []TicketView `json:"tickets"`
	Recipes        []RecipeView `json:"recipes"`
	CanRecord      bool         `json:"canRecord"`
	CanEditRecipes bool         `json:"canEditRecipes"`
}

type RecipeDraft struct {
	ID      string
	Name    string
	Station string
	Lines   []RecipeLineView
}

func (s *Service) Get(ctx context.Context, ac actor.Context, date string) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.ViewCafe) {
		return View{}, errx.Forbidden("forbidden", "cannot view waste")
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

func (s *Service) Record(ctx context.Context, ac actor.Context, date, station, item, reason, note, recipeID string, grams, cost int64) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.RecordWaste) {
		return View{}, errx.Forbidden("forbidden", "cannot record waste")
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
	var rid id.ID
	if recipeID != "" {
		parsed, err := id.Parse(recipeID)
		if err != nil {
			return View{}, errx.Invalid("unknown_recipe", "recipe is not recorded")
		}
		book, err := s.book(ctx, ac.Workspace.ID)
		if err != nil {
			return View{}, err
		}
		if _, ok := book.Recipe(parsed); !ok {
			return View{}, errx.Invalid("unknown_recipe", "recipe is not recorded")
		}
		rid = parsed
	}
	ticket, err := cafe.NewWasteTicket(ac.Workspace.ID, uid, day, station, item, reason, note, rid, grams, cost, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.CreateWasteTicket(ctx, ticket); err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac, def, day)
}

func (s *Service) SaveRecipes(ctx context.Context, ac actor.Context, in []RecipeDraft) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !canEditRecipes(ac.Scope) {
		return View{}, errx.Forbidden("forbidden", "cannot write recipes")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	items := make([]cafe.Recipe, 0, len(in))
	for _, row := range in {
		rec := cafe.Recipe{Name: row.Name, Station: cafe.PrepStation(row.Station)}
		if row.ID != "" {
			parsed, err := id.Parse(row.ID)
			if err != nil {
				return View{}, errx.Invalid("invalid_recipe", "recipe is not recorded")
			}
			rec.ID = parsed
		}
		lines := make([]cafe.RecipeLine, 0, len(row.Lines))
		for _, line := range row.Lines {
			lines = append(lines, cafe.RecipeLine{Title: line.Title, Grams: line.Grams})
		}
		rec.Lines = lines
		items = append(items, rec)
	}
	book, err := cafe.ReplaceRecipes(ac.Workspace.ID, uid, items, s.clock.Now().UTC())
	if err != nil {
		return View{}, err
	}
	if err := s.store.SaveRecipeBook(ctx, book); err != nil {
		return View{}, err
	}
	def, err := s.definition(ctx, ac)
	if err != nil {
		return View{}, err
	}
	day := finance.BusinessDate(s.clock.Now(), def.Timezone)
	return s.toView(ctx, ac, def, day)
}

func (s *Service) toView(ctx context.Context, ac actor.Context, def cafe.Definition, day string) (View, error) {
	tickets, err := s.store.ListWasteTickets(ctx, ac.Workspace.ID, day)
	if err != nil {
		return View{}, err
	}
	sort.SliceStable(tickets, func(i, j int) bool {
		return tickets[i].RecordedAt.After(tickets[j].RecordedAt)
	})
	sum := cafe.SummarizeWaste(day, tickets)
	view := View{
		BusinessDate:   day,
		Timezone:       def.Timezone,
		Status:         "unavailable",
		ReasonKey:      "dash.wasteNotWired",
		Tickets:        make([]TicketView, 0, len(tickets)),
		Recipes:        []RecipeView{},
		CanRecord:      rbac.Allows(ac.Scope, rbac.RecordWaste),
		CanEditRecipes: canEditRecipes(ac.Scope),
	}
	if sum.Live() {
		view.Status = "live"
		view.ReasonKey = ""
		view.TicketCount = sum.TicketCount
		view.Grams = sum.Grams
		view.Cost = sum.Cost
		view.KitchenGrams = sum.KitchenGrams
		view.BarGrams = sum.BarGrams
	}
	for _, t := range tickets {
		row := TicketView{
			ID: t.ID.String(), Item: t.Item, Station: string(t.Station), Grams: t.Grams, Cost: t.Cost,
			Reason: string(t.Reason), Note: t.Note, RecordedAt: t.RecordedAt.UTC().Format(time.RFC3339),
			BusinessDate: t.BusinessDate,
		}
		if t.RecipeID != id.Zero() {
			row.RecipeID = t.RecipeID.String()
		}
		view.Tickets = append(view.Tickets, row)
	}
	book, err := s.book(ctx, ac.Workspace.ID)
	if err != nil {
		return View{}, err
	}
	view.Recipes = make([]RecipeView, 0, len(book.Items))
	for _, rec := range book.Items {
		lines := make([]RecipeLineView, 0, len(rec.Lines))
		for _, line := range rec.Lines {
			lines = append(lines, RecipeLineView{Title: line.Title, Grams: line.Grams})
		}
		view.Recipes = append(view.Recipes, RecipeView{
			ID: rec.ID.String(), Name: rec.Name, Station: string(rec.Station), Lines: lines,
		})
	}
	return view, nil
}

func (s *Service) book(ctx context.Context, workspaceID id.ID) (cafe.RecipeBook, error) {
	book, err := s.store.GetRecipeBook(ctx, workspaceID)
	if err == nil {
		return book, nil
	}
	if errors.Is(err, errx.ErrNotFound) {
		return cafe.RecipeBook{WorkspaceID: workspaceID, Items: []cafe.Recipe{}}, nil
	}
	return cafe.RecipeBook{}, err
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

func canEditRecipes(scope staffscope.Kind) bool {
	return scope == staffscope.GeneralManager || scope == staffscope.DepartmentManager
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "waste is only available on a cafe workspace")
	}
	return nil
}
