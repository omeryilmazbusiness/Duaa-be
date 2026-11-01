package opsreport

import (
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	KindBreakage = "breakage"
	KindRequest  = "request"
)

const (
	StatusOpen     = "open"
	StatusAssigned = "assigned"
	StatusResolved = "resolved"
)

const (
	ActionAssign  = "assign"
	ActionResolve = "resolve"
)

// Report is a floor signal from staff. GM turns it into a task or closes it.
type Report struct {
	ID             id.ID
	WorkspaceID    id.ID
	ReporterUserID id.ID
	Kind           string
	Title          string
	Detail         string
	Status         string
	LinkedTaskID   id.ID
	AssigneeUserID id.ID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func ParseKind(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "", KindBreakage:
		return KindBreakage, nil
	case KindRequest:
		return KindRequest, nil
	default:
		return "", errx.Invalid("invalid_kind", "kind must be breakage or request")
	}
}

func ParseAction(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case ActionAssign, ActionResolve:
		return strings.TrimSpace(raw), nil
	default:
		return "", errx.Invalid("invalid_action", "action must be assign or resolve")
	}
}

func New(workspaceID, reporter id.ID, kind, title, detail string, now time.Time) (Report, error) {
	title = strings.TrimSpace(title)
	if workspaceID == id.Zero() || reporter == id.Zero() {
		return Report{}, errx.Invalid("invalid_report_refs", "workspace and reporter are required")
	}
	if title == "" {
		return Report{}, errx.Invalid("invalid_title", "report title is required")
	}
	k, err := ParseKind(kind)
	if err != nil {
		return Report{}, err
	}
	return Report{
		ID:             id.New(),
		WorkspaceID:    workspaceID,
		ReporterUserID: reporter,
		Kind:           k,
		Title:          title,
		Detail:         strings.TrimSpace(detail),
		Status:         StatusOpen,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (r Report) Open() bool {
	return r.Status == StatusOpen || r.Status == StatusAssigned
}

func (r *Report) Assign(assignee, taskID id.ID, now time.Time) error {
	if r.Status == StatusResolved {
		return errx.Invalid("report_closed", "resolved reports cannot be assigned")
	}
	if assignee == id.Zero() || taskID == id.Zero() {
		return errx.Invalid("invalid_assignment", "assignee and task are required")
	}
	r.Status = StatusAssigned
	r.AssigneeUserID = assignee
	r.LinkedTaskID = taskID
	r.UpdatedAt = now
	return nil
}

func (r *Report) Resolve(now time.Time) error {
	if r.Status == StatusResolved {
		return nil
	}
	r.Status = StatusResolved
	r.UpdatedAt = now
	return nil
}
