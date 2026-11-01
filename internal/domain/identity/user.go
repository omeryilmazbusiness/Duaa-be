package identity

import (
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	RoleManager    = "manager"
	RolePersonnel  = "personnel"
)

type User struct {
	ID           id.ID
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Membership struct {
	ID             id.ID
	WorkspaceID    id.ID
	UserID         id.ID
	Role           string
	Department     string // empty for general manager
	EmployeeNumber string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || !strings.Contains(email, "@") {
		return "", errx.Invalid("invalid_email", "a valid email is required")
	}
	return email, nil
}

func NormalizeEmployeeNumber(raw string) (string, error) {
	n := strings.TrimSpace(raw)
	if len(n) != 4 {
		return "", errx.Invalid("invalid_employee_number", "employee number must be exactly 4 digits")
	}
	for _, r := range n {
		if r < '0' || r > '9' {
			return "", errx.Invalid("invalid_employee_number", "employee number must be exactly 4 digits")
		}
	}
	return n, nil
}

func PlaceholderEmail(workspaceID id.ID, employeeNumber string) string {
	return "emp+" + workspaceID.String() + "-" + employeeNumber + "@staff.internal"
}

func ValidateRole(role string) error {
	if role != RoleManager && role != RolePersonnel {
		return errx.Invalid("invalid_role", "role must be manager or personnel")
	}
	return nil
}

func DisplayName(first, last string) string {
	name := strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
	return name
}
