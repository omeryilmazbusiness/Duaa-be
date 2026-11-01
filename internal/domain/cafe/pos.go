package cafe

import (
	"strings"
	"time"
	"unicode"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type TicketKind string
type TicketTender string

const (
	TicketSale   TicketKind = "sale"
	TicketReturn TicketKind = "return"
)

const (
	TenderCash  TicketTender = "cash"
	TenderCard  TicketTender = "card"
	TenderOther TicketTender = "other"
)

const (
	MaxPOSRef   = 80
	MaxPOSLines = 200
)

// Ticket is one recorded POS fact. It is never invented by the kernel.
type Ticket struct {
	ID           id.ID
	WorkspaceID  id.ID
	RecordedBy   id.ID
	ExternalRef  string
	BusinessDate string
	Kind         TicketKind
	Lines        int
	Gross        int64
	Discount     int64
	Net          int64
	Tender       TicketTender
	RecordedAt   time.Time
}

// POSClose freezes a day's recorded tickets. An empty day cannot close as a live zero.
type POSClose struct {
	ID           id.ID
	WorkspaceID  id.ID
	ClosedBy     id.ID
	BusinessDate string
	TicketCount  int
	SaleCount    int
	ReturnCount  int
	Gross        int64
	Discount     int64
	Returns      int64
	Net          int64
	Basket       int64
	ClosedAt     time.Time
}

// DaySales is derived only from recorded tickets or a frozen close.
type DaySales struct {
	BusinessDate string
	TicketCount  int
	SaleCount    int
	ReturnCount  int
	Gross        int64
	Discount     int64
	Returns      int64
	Net          int64
	Basket       int64
	Cash         int64
	Card         int64
	Other        int64
	Closed       bool
}

func ParseTicketKind(raw string) (TicketKind, error) {
	switch TicketKind(strings.ToLower(strings.TrimSpace(raw))) {
	case TicketSale, TicketReturn:
		return TicketKind(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", errx.Invalid("invalid_ticket_kind", "ticket must be a sale or a return")
	}
}

func ParseTicketTender(raw string) (TicketTender, error) {
	switch TicketTender(strings.ToLower(strings.TrimSpace(raw))) {
	case TenderCash, TenderCard, TenderOther:
		return TicketTender(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", errx.Invalid("invalid_tender", "tender must be cash, card or other")
	}
}

func NormalizePOSRef(raw string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(raw))
}

func NewTicket(workspaceID, recorder id.ID, date, externalRef, kind, tender string, lines int, gross, discount int64, now time.Time) (Ticket, error) {
	if workspaceID == id.Zero() || recorder == id.Zero() {
		return Ticket{}, errx.Invalid("invalid_ticket_refs", "workspace and recorder are required")
	}
	if !validBusinessDate(date) {
		return Ticket{}, errx.Invalid("invalid_business_date", "business date must be YYYY-MM-DD")
	}
	ref := NormalizePOSRef(externalRef)
	if ref == "" {
		return Ticket{}, errx.Invalid("invalid_ticket_ref", "POS ticket reference is required")
	}
	if len(ref) > MaxPOSRef {
		return Ticket{}, errx.Invalid("ticket_ref_too_long", "POS ticket reference is too long")
	}
	parsedKind, err := ParseTicketKind(kind)
	if err != nil {
		return Ticket{}, err
	}
	parsedTender, err := ParseTicketTender(tender)
	if err != nil {
		return Ticket{}, err
	}
	if lines < 1 || lines > MaxPOSLines {
		return Ticket{}, errx.Invalid("invalid_ticket_lines", "ticket must have between 1 and 200 lines")
	}
	if gross < 0 || discount < 0 {
		return Ticket{}, errx.Invalid("invalid_ticket_amount", "amounts must be zero or more")
	}
	if discount > gross {
		return Ticket{}, errx.Invalid("invalid_ticket_discount", "discount cannot exceed gross")
	}
	return Ticket{
		ID:           id.New(),
		WorkspaceID:  workspaceID,
		RecordedBy:   recorder,
		ExternalRef:  ref,
		BusinessDate: date,
		Kind:         parsedKind,
		Lines:        lines,
		Gross:        gross,
		Discount:     discount,
		Net:          gross - discount,
		Tender:       parsedTender,
		RecordedAt:   now,
	}, nil
}

func SummarizeTickets(date string, tickets []Ticket) DaySales {
	out := DaySales{BusinessDate: date}
	for _, t := range tickets {
		if t.WorkspaceID == id.Zero() || t.BusinessDate != date {
			continue
		}
		out.TicketCount++
		switch t.Kind {
		case TicketSale:
			out.SaleCount++
			out.Gross += t.Gross
			out.Discount += t.Discount
			out.Net += t.Net
			addTender(&out, t.Tender, t.Net)
		case TicketReturn:
			out.ReturnCount++
			out.Returns += t.Net
			out.Net -= t.Net
			addTender(&out, t.Tender, -t.Net)
		}
	}
	if out.SaleCount > 0 {
		out.Basket = (out.Gross - out.Discount) / int64(out.SaleCount)
	}
	return out
}

func (s DaySales) Live() bool {
	return s.TicketCount > 0 || s.Closed
}

func ClosePOSDay(workspaceID, closer id.ID, date string, tickets []Ticket, now time.Time) (POSClose, error) {
	if workspaceID == id.Zero() || closer == id.Zero() {
		return POSClose{}, errx.Invalid("invalid_pos_close_refs", "workspace and closer are required")
	}
	if !validBusinessDate(date) {
		return POSClose{}, errx.Invalid("invalid_business_date", "business date must be YYYY-MM-DD")
	}
	day := SummarizeTickets(date, tickets)
	if day.TicketCount == 0 {
		return POSClose{}, errx.Invalid("empty_pos_day", "a POS day with no tickets cannot close as a live zero")
	}
	return POSClose{
		ID:           id.New(),
		WorkspaceID:  workspaceID,
		ClosedBy:     closer,
		BusinessDate: date,
		TicketCount:  day.TicketCount,
		SaleCount:    day.SaleCount,
		ReturnCount:  day.ReturnCount,
		Gross:        day.Gross,
		Discount:     day.Discount,
		Returns:      day.Returns,
		Net:          day.Net,
		Basket:       day.Basket,
		ClosedAt:     now,
	}, nil
}

func (c POSClose) DaySales() DaySales {
	return DaySales{
		BusinessDate: c.BusinessDate,
		TicketCount:  c.TicketCount,
		SaleCount:    c.SaleCount,
		ReturnCount:  c.ReturnCount,
		Gross:        c.Gross,
		Discount:     c.Discount,
		Returns:      c.Returns,
		Net:          c.Net,
		Basket:       c.Basket,
		Closed:       true,
	}
}

func DuplicateTicketConflict() error {
	return errx.Conflict("duplicate_ticket", "this POS ticket is already recorded")
}

func POSDayClosedConflict() error {
	return errx.Conflict("pos_day_closed", "this POS day is already closed")
}

func POSNotUsedError() error {
	return errx.Invalid("pos_not_used", "this cafe has no POS")
}

func addTender(day *DaySales, tender TicketTender, amount int64) {
	switch tender {
	case TenderCash:
		day.Cash += amount
	case TenderCard:
		day.Card += amount
	default:
		day.Other += amount
	}
}

func validBusinessDate(raw string) bool {
	if len(raw) != 10 || raw[4] != '-' || raw[7] != '-' {
		return false
	}
	_, err := time.Parse("2006-01-02", raw)
	return err == nil
}
