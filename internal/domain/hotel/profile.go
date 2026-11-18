package hotel

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	MaxProfileName    = 120
	MaxProfileCity    = 80
	MaxProfileAddress = 240
	MaxProfilePhone   = 40
	MaxProfileGM      = 120
	MaxProfileNotes   = 1000
	MaxStarRating     = 5
)

// Profile is the executive-editable identity of one hotel workspace.
// DisplayName and Timezone mirror the workspace row so every screen agrees.
type Profile struct {
	WorkspaceID id.ID     `json:"workspaceId"`
	DisplayName string    `json:"displayName"`
	TotalRooms  int       `json:"totalRooms"`
	Currency    string    `json:"currency"`
	Timezone    string    `json:"timezone"`
	StarRating  int       `json:"starRating"`
	City        string    `json:"city"`
	Address     string    `json:"address"`
	Phone       string    `json:"phone"`
	GMName      string    `json:"gmName"`
	Notes       string    `json:"notes"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UpdatedBy   id.ID     `json:"updatedBy"`
}

func (p *Profile) Normalize() {
	p.DisplayName = collapseSpaces(p.DisplayName)
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	if p.Currency == "" {
		p.Currency = "USD"
	}
	p.Timezone = strings.TrimSpace(p.Timezone)
	p.City = collapseSpaces(p.City)
	p.Address = collapseSpaces(p.Address)
	p.Phone = collapseSpaces(p.Phone)
	p.GMName = collapseSpaces(p.GMName)
	p.Notes = strings.TrimSpace(p.Notes)
}

func (p Profile) Validate() error {
	if p.WorkspaceID == id.Zero() {
		return errx.Invalid("invalid_workspace", "workspace is required")
	}
	if p.DisplayName == "" {
		return errx.Invalid("hotel_name_required", "hotel name is required")
	}
	if err := maxRunes("hotel_name_too_long", "hotel name", p.DisplayName, MaxProfileName); err != nil {
		return err
	}
	if p.TotalRooms < 0 || p.TotalRooms > MaxHotelRooms {
		return errx.Invalid("invalid_total_rooms", "total rooms must be between 0 and 10000")
	}
	if !validCurrency(p.Currency) {
		return errx.Invalid("invalid_currency", "currency must be a 3-letter ISO code")
	}
	if p.Timezone != "" {
		if _, err := time.LoadLocation(p.Timezone); err != nil {
			return errx.Invalid("invalid_timezone", "timezone must be an IANA name such as Asia/Baghdad")
		}
	}
	if p.StarRating < 0 || p.StarRating > MaxStarRating {
		return errx.Invalid("invalid_star_rating", "star rating must be between 0 and 5")
	}
	for _, f := range []struct {
		code, label, val string
		max              int
	}{
		{"hotel_city_too_long", "city", p.City, MaxProfileCity},
		{"hotel_address_too_long", "address", p.Address, MaxProfileAddress},
		{"hotel_phone_too_long", "phone", p.Phone, MaxProfilePhone},
		{"hotel_gm_too_long", "general manager", p.GMName, MaxProfileGM},
		{"hotel_notes_too_long", "notes", p.Notes, MaxProfileNotes},
	} {
		if err := maxRunes(f.code, f.label, f.val, f.max); err != nil {
			return err
		}
	}
	return nil
}

func ProfileNotFoundError() error {
	return errx.NotFound("hotel_profile_not_found", "hotel profile not found")
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func maxRunes(code, label, val string, max int) error {
	if utf8.RuneCountInString(val) > max {
		return errx.Invalid(code, label+" is too long")
	}
	return nil
}

func validCurrency(c string) bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
