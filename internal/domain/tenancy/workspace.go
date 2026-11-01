package tenancy

import (
	"strings"
	"time"
	"unicode"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Organization struct {
	ID        id.ID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Workspace struct {
	ID             id.ID
	OrganizationID id.ID
	Kind           Kind
	Name           string
	Slug           string
	Timezone       string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NormalizeSlug(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", errx.Invalid("invalid_slug", "slug is required")
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) || r == '_' {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "", errx.Invalid("invalid_slug", "slug must contain letters or digits")
	}
	return out, nil
}

func NewWorkspace(orgID id.ID, kind Kind, name, slug, timezone string, now time.Time) (Workspace, error) {
	if orgID == id.Zero() {
		return Workspace{}, errx.Invalid("invalid_organization", "organization is required")
	}
	if !kind.Valid() {
		return Workspace{}, errx.Invalid("invalid_workspace_kind", "unknown workspace kind")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, errx.Invalid("invalid_name", "workspace name is required")
	}
	normalized, err := NormalizeSlug(slug)
	if err != nil {
		return Workspace{}, err
	}
	if timezone == "" {
		timezone = "UTC"
	}
	return Workspace{
		ID:             id.New(),
		OrganizationID: orgID,
		Kind:           kind,
		Name:           name,
		Slug:           normalized,
		Timezone:       timezone,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}
