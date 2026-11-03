package cafe

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	MaxRecipes     = 24
	MaxRecipeLines = 12
	MaxRecipeName  = 80
	MinRecipeName  = 2
	MaxRecipeGrams = 1_000_000
)

// RecipeLine is one recorded ingredient weight. Grams are never invented.
type RecipeLine struct {
	Title string
	Grams int64
}

// Recipe is a lab card. It does not compute waste or money.
type Recipe struct {
	ID      id.ID
	Name    string
	Station PrepStation
	Lines   []RecipeLine
}

// RecipeBook is the recorded kitchen/bar cards. Empty is valid.
type RecipeBook struct {
	WorkspaceID id.ID
	Items       []Recipe
	UpdatedAt   time.Time
	UpdatedBy   id.ID
}

func NormalizeRecipeName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func (b *RecipeBook) Normalize() {
	if b == nil {
		return
	}
	items := make([]Recipe, 0, len(b.Items))
	seenID := map[id.ID]struct{}{}
	seenName := map[string]struct{}{}
	for _, rec := range b.Items {
		rec.Name = clip(strings.TrimSpace(rec.Name), MaxRecipeName)
		station, err := ParsePrepStation(string(rec.Station))
		if err != nil {
			continue
		}
		rec.Station = station
		if rec.ID == id.Zero() {
			rec.ID = id.New()
		}
		if _, ok := seenID[rec.ID]; ok {
			continue
		}
		key := NormalizeRecipeName(rec.Name)
		if key == "" {
			continue
		}
		if _, ok := seenName[key]; ok {
			continue
		}
		lines := make([]RecipeLine, 0, len(rec.Lines))
		for _, line := range rec.Lines {
			title := clip(strings.TrimSpace(line.Title), MaxRecipeName)
			if title == "" || line.Grams < 1 || line.Grams > MaxRecipeGrams {
				continue
			}
			lines = append(lines, RecipeLine{Title: title, Grams: line.Grams})
			if len(lines) == MaxRecipeLines {
				break
			}
		}
		rec.Lines = lines
		seenID[rec.ID] = struct{}{}
		seenName[key] = struct{}{}
		items = append(items, rec)
		if len(items) == MaxRecipes {
			break
		}
	}
	b.Items = items
}

func (b RecipeBook) Validate() error {
	if b.WorkspaceID == id.Zero() || b.UpdatedBy == id.Zero() {
		return errx.Invalid("invalid_recipe_book", "workspace and author are required")
	}
	if len(b.Items) > MaxRecipes {
		return errx.Invalid("recipes_too_long", "too many recipes")
	}
	for _, rec := range b.Items {
		if utf8.RuneCountInString(rec.Name) < MinRecipeName {
			return errx.Invalid("invalid_recipe_name", "write the recipe name")
		}
		if _, err := ParsePrepStation(string(rec.Station)); err != nil {
			return err
		}
		if len(rec.Lines) == 0 {
			return errx.Invalid("recipe_needs_lines", "a recipe needs at least one ingredient")
		}
		if len(rec.Lines) > MaxRecipeLines {
			return errx.Invalid("recipe_too_long", "too many ingredient lines")
		}
		for _, line := range rec.Lines {
			if utf8.RuneCountInString(line.Title) < MinRecipeName {
				return errx.Invalid("invalid_recipe_line", "write the ingredient")
			}
			if line.Grams < 1 || line.Grams > MaxRecipeGrams {
				return errx.Invalid("invalid_recipe_grams", "ingredient grams must be recorded")
			}
		}
	}
	return nil
}

func (b RecipeBook) Recipe(id id.ID) (Recipe, bool) {
	for _, rec := range b.Items {
		if rec.ID == id {
			return rec, true
		}
	}
	return Recipe{}, false
}

func ReplaceRecipes(workspaceID, actor id.ID, items []Recipe, now time.Time) (RecipeBook, error) {
	book := RecipeBook{
		WorkspaceID: workspaceID,
		Items:       items,
		UpdatedAt:   now,
		UpdatedBy:   actor,
	}
	book.Normalize()
	if err := book.Validate(); err != nil {
		return RecipeBook{}, err
	}
	return book, nil
}

func RecipeBookNotFoundError() error {
	return errx.NotFound("recipe_book_not_found", "recipe book not found")
}
