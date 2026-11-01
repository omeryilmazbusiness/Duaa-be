package cafe

import (
	"crypto/rand"
	"encoding/base64"
	"regexp"
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const TokenLen = 22

var tokenPat = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

// Code is the public scan identity of one QR site. The token is random,
// not sequential, and dies on rotate.
type Code struct {
	WorkspaceID id.ID
	SiteID      id.ID
	Token       string
	IssuedAt    time.Time
	RotatedAt   time.Time
}

func NewToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func ParseToken(raw string) (string, error) {
	tok := strings.TrimSpace(raw)
	if !tokenPat.MatchString(tok) {
		return "", errx.Invalid("invalid_qr_token", "QR token is invalid")
	}
	return tok, nil
}

func (c Code) Validate() error {
	if c.WorkspaceID == id.Zero() || c.SiteID == id.Zero() {
		return errx.Invalid("invalid_qr_code", "workspace and site are required")
	}
	if _, err := ParseToken(c.Token); err != nil {
		return err
	}
	if c.IssuedAt.IsZero() {
		return errx.Invalid("invalid_qr_code", "issued time is required")
	}
	return nil
}

func PublicPath(token string) string {
	return "/q/" + token
}

func PublicURL(appBase, token string) string {
	base := strings.TrimRight(strings.TrimSpace(appBase), "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + PublicPath(token)
}
