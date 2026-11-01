package finance

import (
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	StatusOpen          = "open"
	StatusApproved      = "approved"
	StatusRejected      = "rejected"
	StatusNeedsEvidence = "needs_evidence"
)

const (
	ActionApprove         = "approve"
	ActionReject          = "reject"
	ActionRequestEvidence = "request_evidence"
)

// Approval is a spend pack. Approving it does not move money.
type Approval struct {
	ID              id.ID
	WorkspaceID     id.ID
	CreatedByUserID id.ID
	Title           string
	Supplier        string
	Amount          int64
	Currency        string
	Evidence        string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func ParseApprovalAction(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case ActionApprove, ActionReject, ActionRequestEvidence:
		return strings.TrimSpace(raw), nil
	default:
		return "", errx.Invalid("invalid_action", "action must be approve, reject or request_evidence")
	}
}

func NewApproval(workspaceID, creator id.ID, title, supplier, currency, evidence string, amount int64, now time.Time) (Approval, error) {
	title = strings.TrimSpace(title)
	if workspaceID == id.Zero() || creator == id.Zero() {
		return Approval{}, errx.Invalid("invalid_approval_refs", "workspace and creator are required")
	}
	if title == "" {
		return Approval{}, errx.Invalid("invalid_title", "approval title is required")
	}
	if amount <= 0 {
		return Approval{}, errx.Invalid("invalid_amount", "amount must be greater than zero")
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = CurrencyIQD
	}
	return Approval{
		ID:              id.New(),
		WorkspaceID:     workspaceID,
		CreatedByUserID: creator,
		Title:           title,
		Supplier:        strings.TrimSpace(supplier),
		Amount:          amount,
		Currency:        currency,
		Evidence:        strings.TrimSpace(evidence),
		Status:          StatusOpen,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (a Approval) Pending() bool {
	return a.Status == StatusOpen || a.Status == StatusNeedsEvidence
}

func (a *Approval) Act(action string, now time.Time) error {
	parsed, err := ParseApprovalAction(action)
	if err != nil {
		return err
	}
	switch parsed {
	case ActionApprove:
		a.Status = StatusApproved
	case ActionReject:
		a.Status = StatusRejected
	case ActionRequestEvidence:
		a.Status = StatusNeedsEvidence
	}
	a.UpdatedAt = now
	return nil
}

func (a *Approval) ReviseAmount(amount int64, now time.Time) error {
	if amount <= 0 {
		return errx.Invalid("invalid_amount", "amount must be greater than zero")
	}
	if amount == a.Amount {
		return nil
	}
	a.Amount = amount
	a.Status = StatusOpen
	a.UpdatedAt = now
	return nil
}
