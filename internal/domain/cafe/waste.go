package cafe

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type WasteReason string

const (
	WastePrep     WasteReason = "prep"
	WasteSpoilage WasteReason = "spoilage"
	WasteOvercook WasteReason = "overcook"
	WasteOther    WasteReason = "other"
)

const (
	MaxWasteItem  = 80
	MinWasteItem  = 2
	MaxWasteNote  = 200
	MaxWasteGrams = 1_000_000
)

// WasteTicket is one recorded fire line. Grams and cost are never invented from POS.
type WasteTicket struct {
	ID           id.ID
	WorkspaceID  id.ID
	RecordedBy   id.ID
	BusinessDate string
	Station      PrepStation
	Item         string
	RecipeID     id.ID
	Grams        int64
	Cost         int64
	Reason       WasteReason
	Note         string
	RecordedAt   time.Time
}

// DayWaste is derived only from recorded waste tickets.
type DayWaste struct {
	BusinessDate string
	TicketCount  int
	Grams        int64
	Cost         int64
	KitchenGrams int64
	BarGrams     int64
}

func ParseWasteReason(raw string) (WasteReason, error) {
	switch WasteReason(strings.ToLower(strings.TrimSpace(raw))) {
	case "":
		return WasteOther, nil
	case WastePrep, WasteSpoilage, WasteOvercook, WasteOther:
		return WasteReason(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", errx.Invalid("invalid_waste_reason", "reason must be prep, spoilage, overcook or other")
	}
}

func NewWasteTicket(workspaceID, recorder id.ID, date, station, item, reason, note string, recipeID id.ID, grams, cost int64, now time.Time) (WasteTicket, error) {
	if workspaceID == id.Zero() || recorder == id.Zero() {
		return WasteTicket{}, errx.Invalid("invalid_waste_refs", "workspace and recorder are required")
	}
	if !validBusinessDate(date) {
		return WasteTicket{}, errx.Invalid("invalid_business_date", "business date must be YYYY-MM-DD")
	}
	parsedStation, err := ParsePrepStation(station)
	if err != nil {
		return WasteTicket{}, err
	}
	name := clip(strings.TrimSpace(item), MaxWasteItem)
	if utf8.RuneCountInString(name) < MinWasteItem {
		return WasteTicket{}, errx.Invalid("invalid_waste_item", "write what was wasted")
	}
	parsedReason, err := ParseWasteReason(reason)
	if err != nil {
		return WasteTicket{}, err
	}
	if grams < 1 || grams > MaxWasteGrams {
		return WasteTicket{}, errx.Invalid("invalid_waste_grams", "waste grams must be recorded")
	}
	if cost < 1 {
		return WasteTicket{}, errx.Invalid("invalid_waste_cost", "waste cost must be recorded")
	}
	if now.IsZero() {
		return WasteTicket{}, errx.Invalid("invalid_waste_clock", "recorded time is required")
	}
	return WasteTicket{
		ID:           id.New(),
		WorkspaceID:  workspaceID,
		RecordedBy:   recorder,
		BusinessDate: date,
		Station:      parsedStation,
		Item:         name,
		RecipeID:     recipeID,
		Grams:        grams,
		Cost:         cost,
		Reason:       parsedReason,
		Note:         clip(strings.TrimSpace(note), MaxWasteNote),
		RecordedAt:   now,
	}, nil
}

func SummarizeWaste(date string, tickets []WasteTicket) DayWaste {
	out := DayWaste{BusinessDate: date}
	for _, t := range tickets {
		if t.WorkspaceID == id.Zero() || t.BusinessDate != date {
			continue
		}
		out.TicketCount++
		out.Grams += t.Grams
		out.Cost += t.Cost
		switch t.Station {
		case StationKitchen:
			out.KitchenGrams += t.Grams
		case StationBar:
			out.BarGrams += t.Grams
		}
	}
	return out
}

func (s DayWaste) Live() bool { return s.TicketCount > 0 }

func WasteNotFoundError() error {
	return errx.NotFound("waste_ticket_not_found", "waste ticket not found")
}
