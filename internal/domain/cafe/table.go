package cafe

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const MaxTableNumber = 16

// ParseTableNumber is the printable name of a seat. The QR token is not derived from it.
func ParseTableNumber(raw string) (string, error) {
	n := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if n == "" {
		return "", errx.Invalid("invalid_table", "table number is required")
	}
	if utf8.RuneCountInString(n) > MaxTableNumber {
		return "", errx.Invalid("invalid_table", "table number is too long")
	}
	hasAtom := false
	for _, r := range n {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasAtom = true
			continue
		}
		if r == '-' || r == '/' || r == '.' || r == ' ' {
			continue
		}
		return "", errx.Invalid("invalid_table", "table number has invalid characters")
	}
	if !hasAtom {
		return "", errx.Invalid("invalid_table", "table number is required")
	}
	return n, nil
}

func SameTable(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func (d Definition) TableSite(number string) (QRSite, bool) {
	n, err := ParseTableNumber(number)
	if err != nil {
		return QRSite{}, false
	}
	for _, q := range d.QRSites {
		if q.Area == AreaTable && SameTable(q.Label, n) {
			return q, true
		}
	}
	return QRSite{}, false
}
