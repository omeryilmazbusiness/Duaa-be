package hotel_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestProfileNormalizeDefaultsAndTrims(t *testing.T) {
	p := hotel.Profile{WorkspaceID: id.New(), DisplayName: "  Sheraton   Baghdad ", Currency: " iqd ", City: " Baghdad  "}
	p.Normalize()
	if p.DisplayName != "Sheraton Baghdad" || p.Currency != "IQD" || p.City != "Baghdad" {
		t.Fatalf("normalize=%+v", p)
	}
	empty := hotel.Profile{WorkspaceID: id.New(), DisplayName: "X"}
	empty.Normalize()
	if empty.Currency != "USD" {
		t.Fatalf("default currency=%q", empty.Currency)
	}
}

func TestProfileValidate(t *testing.T) {
	ok := hotel.Profile{WorkspaceID: id.New(), DisplayName: "Al Shohada", TotalRooms: 120, Currency: "IQD", Timezone: "Asia/Baghdad", StarRating: 4}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
	cases := map[string]func(p *hotel.Profile){
		"hotel_name_required":    func(p *hotel.Profile) { p.DisplayName = "" },
		"hotel_name_too_long":    func(p *hotel.Profile) { p.DisplayName = strings.Repeat("a", hotel.MaxProfileName+1) },
		"invalid_total_rooms":    func(p *hotel.Profile) { p.TotalRooms = hotel.MaxHotelRooms + 1 },
		"invalid_currency":       func(p *hotel.Profile) { p.Currency = "US1" },
		"invalid_timezone":       func(p *hotel.Profile) { p.Timezone = "Mars/Olympus" },
		"invalid_star_rating":    func(p *hotel.Profile) { p.StarRating = 6 },
		"hotel_notes_too_long":   func(p *hotel.Profile) { p.Notes = strings.Repeat("n", hotel.MaxProfileNotes+1) },
		"hotel_address_too_long": func(p *hotel.Profile) { p.Address = strings.Repeat("a", hotel.MaxProfileAddress+1) },
		"invalid_workspace":      func(p *hotel.Profile) { p.WorkspaceID = id.Zero() },
	}
	for code, mutate := range cases {
		p := ok
		mutate(&p)
		err := p.Validate()
		if err == nil || !errors.Is(err, errx.ErrInvalid) || !strings.Contains(errorCode(err), code) {
			t.Errorf("%s: got %v", code, err)
		}
	}
}

func errorCode(err error) string {
	var e *errx.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return err.Error()
}
