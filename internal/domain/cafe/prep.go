package cafe

import (
	"strings"
	"time"
	"unicode"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type PrepStation string
type PrepStatus string

const (
	StationKitchen PrepStation = "kitchen"
	StationBar     PrepStation = "bar"
)

const (
	PrepWaiting PrepStatus = "waiting"
	PrepReady   PrepStatus = "ready"
	PrepServed  PrepStatus = "served"
)

const MaxPrepRef = 80

// PrepTicket is one recorded kitchen/bar order clock. Times are never invented.
type PrepTicket struct {
	ID           id.ID
	WorkspaceID  id.ID
	OpenedBy     id.ID
	ReadyBy      id.ID
	ServedBy     id.ID
	BusinessDate string
	Station      PrepStation
	TicketRef    string
	TableLabel   string
	ReceivedAt   time.Time
	ReadyAt      time.Time
	ServedAt     time.Time
}

// DayPrep is derived only from recorded prep tickets.
type DayPrep struct {
	BusinessDate       string
	TicketCount        int
	WaitingCount       int
	ReadyCount         int
	ServedCount        int
	LongestOpenMinutes int
}

func ParsePrepStation(raw string) (PrepStation, error) {
	switch PrepStation(strings.ToLower(strings.TrimSpace(raw))) {
	case StationKitchen, StationBar:
		return PrepStation(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", errx.Invalid("invalid_prep_station", "station must be kitchen or bar")
	}
}

func NormalizePrepRef(raw string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(raw))
}

func NewPrepTicket(workspaceID, opener id.ID, date, station, ticketRef, tableLabel string, now time.Time) (PrepTicket, error) {
	if workspaceID == id.Zero() || opener == id.Zero() {
		return PrepTicket{}, errx.Invalid("invalid_prep_refs", "workspace and opener are required")
	}
	if !validBusinessDate(date) {
		return PrepTicket{}, errx.Invalid("invalid_business_date", "business date must be YYYY-MM-DD")
	}
	parsedStation, err := ParsePrepStation(station)
	if err != nil {
		return PrepTicket{}, err
	}
	ref := NormalizePrepRef(ticketRef)
	if ref == "" {
		return PrepTicket{}, errx.Invalid("invalid_prep_ref", "prep ticket reference is required")
	}
	if len(ref) > MaxPrepRef {
		return PrepTicket{}, errx.Invalid("prep_ref_too_long", "prep ticket reference is too long")
	}
	table := strings.TrimSpace(tableLabel)
	if table != "" {
		parsed, err := ParseTableNumber(table)
		if err != nil {
			return PrepTicket{}, err
		}
		table = parsed
	}
	if now.IsZero() {
		return PrepTicket{}, errx.Invalid("invalid_prep_clock", "received time is required")
	}
	return PrepTicket{
		ID:           id.New(),
		WorkspaceID:  workspaceID,
		OpenedBy:     opener,
		BusinessDate: date,
		Station:      parsedStation,
		TicketRef:    ref,
		TableLabel:   table,
		ReceivedAt:   now,
	}, nil
}

func (t PrepTicket) Status() PrepStatus {
	if !t.ServedAt.IsZero() {
		return PrepServed
	}
	if !t.ReadyAt.IsZero() {
		return PrepReady
	}
	return PrepWaiting
}

func (t PrepTicket) Open() bool {
	return t.Status() != PrepServed
}

func (t *PrepTicket) MarkReady(actor id.ID, now time.Time) error {
	if t.Status() != PrepWaiting {
		return errx.Invalid("prep_not_waiting", "only a waiting ticket can be marked ready")
	}
	if actor == id.Zero() {
		return errx.Invalid("invalid_prep_actor", "who marked ready is required")
	}
	if now.Before(t.ReceivedAt) {
		return errx.Invalid("invalid_prep_clock", "ready cannot precede received")
	}
	t.ReadyAt = now
	t.ReadyBy = actor
	return nil
}

func (t *PrepTicket) MarkServed(actor id.ID, now time.Time) error {
	if t.Status() != PrepReady {
		return errx.Invalid("prep_not_ready", "only a ready ticket can be marked served")
	}
	if actor == id.Zero() {
		return errx.Invalid("invalid_prep_actor", "who marked served is required")
	}
	if now.Before(t.ReadyAt) {
		return errx.Invalid("invalid_prep_clock", "served cannot precede ready")
	}
	t.ServedAt = now
	t.ServedBy = actor
	return nil
}

func (t PrepTicket) WaitMinutes(now time.Time) int {
	end := now
	if !t.ReadyAt.IsZero() {
		end = t.ReadyAt
	}
	return elapsedMinutes(t.ReceivedAt, end)
}

func (t PrepTicket) OpenMinutes(now time.Time) int {
	end := now
	if !t.ServedAt.IsZero() {
		end = t.ServedAt
	}
	return elapsedMinutes(t.ReceivedAt, end)
}

func SummarizePrep(date string, tickets []PrepTicket, now time.Time) DayPrep {
	out := DayPrep{BusinessDate: date}
	for _, t := range tickets {
		if t.WorkspaceID == id.Zero() || t.BusinessDate != date {
			continue
		}
		out.TicketCount++
		switch t.Status() {
		case PrepWaiting:
			out.WaitingCount++
		case PrepReady:
			out.ReadyCount++
		case PrepServed:
			out.ServedCount++
		}
		if t.Open() {
			if open := t.OpenMinutes(now); open > out.LongestOpenMinutes {
				out.LongestOpenMinutes = open
			}
		}
	}
	return out
}

func (s DayPrep) Live() bool { return s.TicketCount > 0 }

func (s DayPrep) OpenCount() int { return s.WaitingCount + s.ReadyCount }

func DuplicatePrepConflict() error {
	return errx.Conflict("duplicate_prep_ticket", "this prep ticket is already recorded")
}

func PrepNotFoundError() error {
	return errx.NotFound("prep_ticket_not_found", "prep ticket not found")
}

func elapsedMinutes(from, to time.Time) int {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return 0
	}
	return int(to.Sub(from) / time.Minute)
}
