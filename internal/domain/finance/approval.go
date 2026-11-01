package finance

import (
	"strconv"
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

const MaxEvidence = 500

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
	Fingerprint     string
	Version         int
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
	evidence = strings.TrimSpace(evidence)
	if len(evidence) > MaxEvidence {
		return Approval{}, errx.Invalid("evidence_too_long", "evidence must be 500 characters or fewer")
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = CurrencyIQD
	}
	a := Approval{
		ID:              id.New(),
		WorkspaceID:     workspaceID,
		CreatedByUserID: creator,
		Title:           title,
		Supplier:        strings.TrimSpace(supplier),
		Amount:          amount,
		Currency:        currency,
		Evidence:        evidence,
		Version:         1,
		Status:          StatusOpen,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	a.refreshFingerprint()
	return a, nil
}

func PackFingerprint(title, supplier string, amount int64, currency string) string {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = CurrencyIQD
	}
	return normalizeToken(title) + "|" + normalizeToken(supplier) + "|" + strconv.FormatInt(amount, 10) + "|" + cur
}

func PendingDuplicate(existing []Approval, candidate Approval) bool {
	key := candidate.Fingerprint
	if key == "" {
		key = PackFingerprint(candidate.Title, candidate.Supplier, candidate.Amount, candidate.Currency)
	}
	for _, a := range existing {
		if a.ID == candidate.ID || a.WorkspaceID != candidate.WorkspaceID || !a.Pending() {
			continue
		}
		got := a.Fingerprint
		if got == "" {
			got = PackFingerprint(a.Title, a.Supplier, a.Amount, a.Currency)
		}
		if got == key {
			return true
		}
	}
	return false
}

func DuplicateConflict() error {
	return errx.Conflict("duplicate_approval", "a pending pack for this invoice already exists")
}

func (a *Approval) refreshFingerprint() {
	a.Fingerprint = PackFingerprint(a.Title, a.Supplier, a.Amount, a.Currency)
}

func (a Approval) Pending() bool {
	return a.Status == StatusOpen || a.Status == StatusNeedsEvidence
}

func (a Approval) Closed() bool {
	return a.Status == StatusApproved || a.Status == StatusRejected
}

func (a Approval) HasEvidence() bool {
	return strings.TrimSpace(a.Evidence) != ""
}

func (a Approval) CanApprove() bool {
	return a.Status == StatusOpen && a.HasEvidence()
}

func (a *Approval) Act(action string, now time.Time) error {
	if a.Closed() {
		return errx.Invalid("approval_closed", "this pack is already decided")
	}
	parsed, err := ParseApprovalAction(action)
	if err != nil {
		return err
	}
	switch parsed {
	case ActionApprove:
		if a.Status == StatusNeedsEvidence || !a.HasEvidence() {
			return errx.Invalid("evidence_required", "attach evidence before approving")
		}
		a.Status = StatusApproved
	case ActionReject:
		a.Status = StatusRejected
	case ActionRequestEvidence:
		a.Status = StatusNeedsEvidence
	}
	a.UpdatedAt = now
	return nil
}

func (a *Approval) AttachEvidence(evidence string, now time.Time) error {
	if a.Closed() {
		return errx.Invalid("approval_closed", "this pack is already decided")
	}
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return errx.Invalid("evidence_required", "evidence is required")
	}
	if len(evidence) > MaxEvidence {
		return errx.Invalid("evidence_too_long", "evidence must be 500 characters or fewer")
	}
	changed := evidence != a.Evidence || a.Status == StatusNeedsEvidence
	a.Evidence = evidence
	if a.Status == StatusNeedsEvidence {
		a.Status = StatusOpen
	}
	if changed {
		a.Version++
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
	a.refreshFingerprint()
	a.Version++
	a.Status = StatusOpen
	a.UpdatedAt = now
	return nil
}

func normalizeToken(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
