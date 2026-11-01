package executive

import (
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	StatusOpen         = "open"
	StatusAcknowledged = "acknowledged"
	StatusAssigned     = "assigned"
	StatusResolved     = "resolved"
)

const (
	ActionAcknowledge = "acknowledge"
	ActionAssign      = "assign"
	ActionResolve     = "resolve"
)

// Decision is the GM write-model. Brief cards are live; this records what the GM did.
type Decision struct {
	ID              id.ID
	WorkspaceID     id.ID
	CreatedByUserID id.ID
	SourceKey       string
	Fingerprint     string
	Kind            string
	TitleKey        string
	Detail          string
	Severity        string
	TargetTab       string
	Status          string
	LinkedTaskID    id.ID
	AssigneeUserID  id.ID
	SnoozeUntil     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewDecision(workspaceID, createdBy id.ID, sourceKey, fingerprint, kind, titleKey, detail, severity, targetTab string, now time.Time) (Decision, error) {
	sourceKey = strings.TrimSpace(sourceKey)
	if workspaceID == id.Zero() || createdBy == id.Zero() {
		return Decision{}, errx.Invalid("invalid_decision_refs", "workspace and creator are required")
	}
	if sourceKey == "" {
		return Decision{}, errx.Invalid("invalid_source", "sourceKey is required")
	}
	return Decision{
		ID:              id.New(),
		WorkspaceID:     workspaceID,
		CreatedByUserID: createdBy,
		SourceKey:       sourceKey,
		Fingerprint:     strings.TrimSpace(fingerprint),
		Kind:            strings.TrimSpace(kind),
		TitleKey:        strings.TrimSpace(titleKey),
		Detail:          strings.TrimSpace(detail),
		Severity:        strings.TrimSpace(severity),
		TargetTab:       strings.TrimSpace(targetTab),
		Status:          StatusOpen,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func ParseAction(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case ActionAcknowledge, ActionAssign, ActionResolve:
		return strings.TrimSpace(raw), nil
	default:
		return "", errx.Invalid("invalid_action", "action must be acknowledge, assign or resolve")
	}
}

func (d Decision) Active() bool {
	return d.Status == StatusOpen || d.Status == StatusAcknowledged || d.Status == StatusAssigned
}

func (d Decision) Can(action string) bool {
	switch action {
	case ActionAcknowledge:
		return d.Status == StatusOpen || d.Status == StatusAcknowledged || d.Status == StatusAssigned
	case ActionAssign:
		return d.Status == StatusOpen || d.Status == StatusAcknowledged || d.Status == StatusAssigned
	case ActionResolve:
		return d.Status == StatusOpen || d.Status == StatusAcknowledged || d.Status == StatusAssigned || d.Status == StatusResolved
	default:
		return false
	}
}

func AllowedActions(status string) []string {
	d := Decision{Status: status}
	if status == "" {
		d.Status = StatusOpen
	}
	out := make([]string, 0, 3)
	for _, action := range []string{ActionAcknowledge, ActionAssign, ActionResolve} {
		if d.Can(action) {
			out = append(out, action)
		}
	}
	return out
}

func (d *Decision) Apply(action string, now, snoozeUntil time.Time) error {
	if !d.Can(action) {
		return errx.Invalid("illegal_transition", "this decision cannot take that action")
	}
	switch action {
	case ActionAcknowledge:
		if d.Status == StatusOpen {
			d.Status = StatusAcknowledged
		}
	case ActionAssign:
		d.Status = StatusAssigned
	case ActionResolve:
		d.Status = StatusResolved
		until := snoozeUntil
		d.SnoozeUntil = &until
	}
	d.UpdatedAt = now
	return nil
}

func (d Decision) HiddenBySnooze(fingerprint string, now time.Time) bool {
	if d.Status != StatusResolved || d.SnoozeUntil == nil {
		return false
	}
	if d.Fingerprint != fingerprint {
		return false
	}
	return now.Before(*d.SnoozeUntil)
}
