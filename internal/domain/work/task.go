package work

import (
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusCancelled  = "cancelled"
	StatusBlocked    = "blocked"
)

type Task struct {
	ID              id.ID
	WorkspaceID     id.ID
	AssigneeUserID  id.ID
	CreatedByUserID id.ID
	Title           string
	Description     string
	ScheduledStart  time.Time
	ScheduledEnd    time.Time
	Status          string
	RoutineID       id.ID
	SourceDate      string
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Routine struct {
	ID              id.ID
	WorkspaceID     id.ID
	CreatedByUserID id.ID
	AssigneeUserID  id.ID
	Title           string
	Description     string
	StartTime       string // HH:MM UTC
	EndTime         string
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewTask(workspaceID, assignee, createdBy id.ID, title, description string, start, end, now time.Time) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, errx.Invalid("invalid_title", "task title is required")
	}
	if workspaceID == id.Zero() || assignee == id.Zero() || createdBy == id.Zero() {
		return Task{}, errx.Invalid("invalid_task_refs", "workspace, assignee and creator are required")
	}
	if !end.After(start) {
		return Task{}, errx.Invalid("invalid_schedule", "task end must be after start")
	}
	return Task{
		ID:              id.New(),
		WorkspaceID:     workspaceID,
		AssigneeUserID:  assignee,
		CreatedByUserID: createdBy,
		Title:           title,
		Description:     strings.TrimSpace(description),
		ScheduledStart:  start.UTC(),
		ScheduledEnd:    end.UTC(),
		Status:          StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (t *Task) Complete(now time.Time) error {
	if t.Status == StatusCancelled {
		return errx.Invalid("task_cancelled", "cancelled tasks cannot be completed")
	}
	if t.Status == StatusCompleted {
		return nil
	}
	t.Status = StatusCompleted
	t.CompletedAt = &now
	t.UpdatedAt = now
	return nil
}

func (t Task) Overdue(now time.Time) bool {
	if t.Status == StatusCompleted || t.Status == StatusCancelled {
		return false
	}
	return now.After(t.ScheduledEnd)
}

func ValidHHMM(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	return h >= 0 && h <= 23 && m >= 0 && m <= 59 && s[0] >= '0' && s[0] <= '9'
}

func NewRoutine(workspaceID, createdBy, assignee id.ID, title, description, start, end string, now time.Time) (Routine, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Routine{}, errx.Invalid("invalid_title", "routine title is required")
	}
	if !ValidHHMM(start) || !ValidHHMM(end) {
		return Routine{}, errx.Invalid("invalid_time", "start and end must be HH:MM")
	}
	return Routine{
		ID:              id.New(),
		WorkspaceID:     workspaceID,
		CreatedByUserID: createdBy,
		AssigneeUserID:  assignee,
		Title:           title,
		Description:     strings.TrimSpace(description),
		StartTime:       start,
		EndTime:         end,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}
