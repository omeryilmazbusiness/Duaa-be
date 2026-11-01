package cafe

import (
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	RecoveryOpen     = "open"
	RecoveryAssigned = "assigned"
	RecoveryClosed   = "closed"
)

const (
	ActionAssign     = "assign"
	ActionCompensate = "compensate"
	ActionClose      = "close"
)

// Recovery is the floor file for one guest voice that needs a reply.
// Compensation is a recorded gesture. It never pays.
type Recovery struct {
	ID                     id.ID
	WorkspaceID            id.ID
	VoiceID                id.ID
	Status                 string
	AssigneeUserID         id.ID
	LinkedTaskID           id.ID
	CompensationAmount     int64
	CompensationCurrency   string
	CompensatedAt          time.Time
	ClosedAt               time.Time
	Note                   string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (v Voice) NeedsRecovery() bool {
	return v.Unhappy() || v.Kind == VoiceRequest
}

func ParseRecoveryAction(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case ActionAssign, ActionCompensate, ActionClose:
		return strings.TrimSpace(raw), nil
	default:
		return "", errx.Invalid("invalid_action", "action must be assign, compensate or close")
	}
}

func OpenRecovery(workspaceID, voiceID id.ID, now time.Time) (Recovery, error) {
	if workspaceID == id.Zero() || voiceID == id.Zero() {
		return Recovery{}, errx.Invalid("invalid_recovery", "workspace and voice are required")
	}
	return Recovery{
		ID:          id.New(),
		WorkspaceID: workspaceID,
		VoiceID:     voiceID,
		Status:      RecoveryOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (r Recovery) Open() bool {
	return r.Status == RecoveryOpen || r.Status == RecoveryAssigned
}

func (r *Recovery) Assign(assignee, taskID id.ID, now time.Time) error {
	if r.Status == RecoveryClosed {
		return errx.Invalid("recovery_closed", "closed recoveries cannot be assigned")
	}
	if assignee == id.Zero() {
		return errx.Invalid("invalid_assignment", "assignee is required")
	}
	r.Status = RecoveryAssigned
	r.AssigneeUserID = assignee
	if taskID != id.Zero() {
		r.LinkedTaskID = taskID
	}
	r.UpdatedAt = now
	return nil
}

func (r *Recovery) Compensate(amount int64, currency string, limit int64, limitCurrency string, now time.Time) error {
	if r.Status == RecoveryClosed {
		return errx.Invalid("recovery_closed", "closed recoveries cannot be compensated")
	}
	if amount <= 0 {
		return errx.Invalid("invalid_compensation", "compensation amount is required")
	}
	if limit <= 0 {
		return errx.Invalid("compensation_not_set", "no compensation limit is recorded")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	limitCurrency = strings.ToUpper(strings.TrimSpace(limitCurrency))
	if currency == "" {
		currency = limitCurrency
	}
	if currency == "" {
		return errx.Invalid("invalid_compensation_currency", "compensation currency is required")
	}
	if limitCurrency != "" && currency != limitCurrency {
		return errx.Invalid("invalid_compensation_currency", "compensation currency must match the SLA")
	}
	if amount > limit {
		return errx.Invalid("compensation_over_limit", "compensation exceeds the recorded limit")
	}
	r.CompensationAmount = amount
	r.CompensationCurrency = currency
	r.CompensatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *Recovery) Close(now time.Time, note string) error {
	if r.Status == RecoveryClosed {
		return nil
	}
	r.Status = RecoveryClosed
	r.ClosedAt = now
	r.Note = clip(strings.TrimSpace(note), MaxComment)
	r.UpdatedAt = now
	return nil
}

type RecoveryClock struct {
	AckDue       time.Time
	ResolveDue   time.Time
	LateAck      bool
	LateResolve  bool
}

func (r Recovery) Clock(sla SLA, opened, now time.Time) RecoveryClock {
	out := RecoveryClock{}
	if sla.AckMinutes > 0 {
		out.AckDue = opened.Add(time.Duration(sla.AckMinutes) * time.Minute)
		if r.Status == RecoveryOpen && now.After(out.AckDue) {
			out.LateAck = true
		}
	}
	if sla.ResolveMinutes > 0 {
		out.ResolveDue = opened.Add(time.Duration(sla.ResolveMinutes) * time.Minute)
		if r.Status != RecoveryClosed && now.After(out.ResolveDue) {
			out.LateResolve = true
		}
	}
	return out
}
