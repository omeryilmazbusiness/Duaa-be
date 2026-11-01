package cafe

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	CheckOpen      CheckKind = "open"
	CheckClose     CheckKind = "close"
	MaxCheckTitle            = 80
	MaxCheckItems            = 24
	MinCheckTitle            = 2
)

// CheckKind is opening or closing. Maintenance photos stay out (CAF-04 remainder).
type CheckKind string

type CheckItem struct {
	ID     id.ID
	Kind   CheckKind
	Title  string
	Sort   int
	Active bool
}

type CheckMark struct {
	ItemID id.ID
	Title  string
	DoneBy id.ID
	DoneAt time.Time
}

// ChecklistTemplate is the recorded open/close list. Empty is valid — never invent lines.
type ChecklistTemplate struct {
	WorkspaceID id.ID
	Items       []CheckItem
	UpdatedAt   time.Time
	UpdatedBy   id.ID
}

// ChecklistRun is today's ticks against the recorded list. Missing run is an empty day.
type ChecklistRun struct {
	ID           id.ID
	WorkspaceID  id.ID
	BusinessDate string
	Kind         CheckKind
	Marks        []CheckMark
	UpdatedAt    time.Time
	UpdatedBy    id.ID
}

func ParseCheckKind(raw string) (CheckKind, error) {
	switch CheckKind(strings.TrimSpace(raw)) {
	case CheckOpen:
		return CheckOpen, nil
	case CheckClose:
		return CheckClose, nil
	case "maintenance":
		return "", errx.Invalid("check_kind_later", "maintenance photos come later")
	default:
		return "", errx.Invalid("invalid_check_kind", "kind must be open or close")
	}
}

func (t *ChecklistTemplate) Normalize() {
	if t == nil {
		return
	}
	items := make([]CheckItem, 0, len(t.Items))
	seen := map[id.ID]struct{}{}
	for i, item := range t.Items {
		item.Title = clip(strings.TrimSpace(item.Title), MaxCheckTitle)
		kind, err := ParseCheckKind(string(item.Kind))
		if err != nil {
			continue
		}
		item.Kind = kind
		if item.Title == "" {
			continue
		}
		if item.ID == id.Zero() {
			item.ID = id.New()
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		if item.Sort == 0 {
			item.Sort = i + 1
		}
		items = append(items, item)
	}
	t.Items = items
}

func (t ChecklistTemplate) Validate() error {
	if t.WorkspaceID == id.Zero() || t.UpdatedBy == id.Zero() {
		return errx.Invalid("invalid_checklist", "workspace and author are required")
	}
	if len(t.Items) > MaxCheckItems {
		return errx.Invalid("checklist_too_long", "too many checklist lines")
	}
	for _, item := range t.Items {
		if utf8.RuneCountInString(item.Title) < MinCheckTitle {
			return errx.Invalid("invalid_check_title", "write the checklist line")
		}
		if _, err := ParseCheckKind(string(item.Kind)); err != nil {
			return err
		}
	}
	return nil
}

func (t ChecklistTemplate) ActiveItems(kind CheckKind) []CheckItem {
	out := make([]CheckItem, 0)
	for _, item := range t.Items {
		if item.Active && item.Kind == kind {
			out = append(out, item)
		}
	}
	return out
}

func (t ChecklistTemplate) Item(id id.ID) (CheckItem, bool) {
	for _, item := range t.Items {
		if item.ID == id {
			return item, true
		}
	}
	return CheckItem{}, false
}

func ReplaceTemplate(workspaceID, actor id.ID, items []CheckItem, now time.Time) (ChecklistTemplate, error) {
	rec := ChecklistTemplate{
		WorkspaceID: workspaceID,
		Items:       items,
		UpdatedAt:   now,
		UpdatedBy:   actor,
	}
	rec.Normalize()
	if err := rec.Validate(); err != nil {
		return ChecklistTemplate{}, err
	}
	return rec, nil
}

func (r *ChecklistRun) Normalize() {
	if r == nil {
		return
	}
	r.BusinessDate = strings.TrimSpace(r.BusinessDate)
	marks := make([]CheckMark, 0, len(r.Marks))
	seen := map[id.ID]struct{}{}
	for _, m := range r.Marks {
		if m.ItemID == id.Zero() {
			continue
		}
		if _, ok := seen[m.ItemID]; ok {
			continue
		}
		seen[m.ItemID] = struct{}{}
		m.Title = clip(strings.TrimSpace(m.Title), MaxCheckTitle)
		marks = append(marks, m)
	}
	r.Marks = marks
	if r.Kind != CheckOpen && r.Kind != CheckClose {
		r.Kind = ""
	}
}

func (r ChecklistRun) Marked(itemID id.ID) bool {
	for _, m := range r.Marks {
		if m.ItemID == itemID {
			return true
		}
	}
	return false
}

func (r ChecklistRun) Remaining(items []CheckItem) int {
	n := 0
	for _, item := range items {
		if !r.Marked(item.ID) {
			n++
		}
	}
	return n
}

func (r ChecklistRun) DoneCount(items []CheckItem) int {
	n := 0
	for _, item := range items {
		if r.Marked(item.ID) {
			n++
		}
	}
	return n
}

func ToggleRun(cur *ChecklistRun, workspaceID, actor id.ID, date string, item CheckItem, now time.Time) (ChecklistRun, error) {
	if !item.Active {
		return ChecklistRun{}, errx.Invalid("check_inactive", "this line is not on today's list")
	}
	if _, err := ParseCheckKind(string(item.Kind)); err != nil {
		return ChecklistRun{}, err
	}
	if item.ID == id.Zero() {
		return ChecklistRun{}, errx.Invalid("unknown_check", "checklist line is not recorded")
	}
	rec := ChecklistRun{
		WorkspaceID:  workspaceID,
		BusinessDate: date,
		Kind:         item.Kind,
		UpdatedAt:    now,
		UpdatedBy:    actor,
	}
	if cur != nil && cur.ID != id.Zero() {
		rec.ID = cur.ID
		rec.Marks = append([]CheckMark(nil), cur.Marks...)
	} else {
		rec.ID = id.New()
	}
	if rec.Marked(item.ID) {
		kept := make([]CheckMark, 0, len(rec.Marks))
		for _, m := range rec.Marks {
			if m.ItemID != item.ID {
				kept = append(kept, m)
			}
		}
		rec.Marks = kept
	} else {
		rec.Marks = append(rec.Marks, CheckMark{
			ItemID: item.ID,
			Title:  clip(strings.TrimSpace(item.Title), MaxCheckTitle),
			DoneBy: actor,
			DoneAt: now,
		})
	}
	rec.Normalize()
	if rec.WorkspaceID == id.Zero() || rec.UpdatedBy == id.Zero() {
		return ChecklistRun{}, errx.Invalid("invalid_checklist", "workspace and author are required")
	}
	if _, err := ParseBusinessDate(rec.BusinessDate); err != nil {
		return ChecklistRun{}, err
	}
	if rec.Kind == "" {
		return ChecklistRun{}, errx.Invalid("invalid_check_kind", "kind must be open or close")
	}
	return rec, nil
}
