package decision

import (
	"context"
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	store  Repository
	tasks  TaskStatusReader
	assign AssignmentPort
	clock  clock.Clock
}

func New(store Repository, tasks TaskStatusReader, assign AssignmentPort, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{store: store, tasks: tasks, assign: assign, clock: clk}
}

type Command struct {
	SourceKey      string
	Fingerprint    string
	Kind           string
	TitleKey       string
	Detail         string
	Severity       string
	TargetTab      string
	Action         string
	AssigneeUserID string
	Title          string
	Description    string
}

type View struct {
	ID             string  `json:"id"`
	SourceKey      string  `json:"sourceKey"`
	Fingerprint    string  `json:"fingerprint"`
	Kind           string  `json:"kind"`
	Status         string  `json:"status"`
	AssigneeUserID string  `json:"assigneeUserId,omitempty"`
	LinkedTaskID   string  `json:"linkedTaskId,omitempty"`
	SnoozeUntil    *string `json:"snoozeUntil,omitempty"`
}

func toView(d executive.Decision) View {
	v := View{
		ID:          d.ID.String(),
		SourceKey:   d.SourceKey,
		Fingerprint: d.Fingerprint,
		Kind:        d.Kind,
		Status:      d.Status,
	}
	if d.AssigneeUserID != id.Zero() {
		v.AssigneeUserID = d.AssigneeUserID.String()
	}
	if d.LinkedTaskID != id.Zero() {
		v.LinkedTaskID = d.LinkedTaskID.String()
	}
	if d.SnoozeUntil != nil {
		s := d.SnoozeUntil.Format(time.RFC3339)
		v.SnoozeUntil = &s
	}
	return v
}

func (s *Service) Reconcile(ctx context.Context, ac actor.Context) error {
	if ac.Scope != staffscope.GeneralManager && ac.Scope != staffscope.DepartmentManager {
		return nil
	}
	list, err := s.store.ListDecisions(ctx, ac.Workspace.ID)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	for _, d := range list {
		if d.WorkspaceID != ac.Workspace.ID || d.Status != executive.StatusAssigned || d.LinkedTaskID == id.Zero() {
			continue
		}
		task, err := s.tasks.GetTask(ctx, d.LinkedTaskID)
		if err != nil {
			continue
		}
		if task.WorkspaceID != ac.Workspace.ID {
			continue
		}
		if task.Status != work.StatusCompleted && task.Status != work.StatusCancelled {
			continue
		}
		// Close the follow-up without snoozing: if the live source is still late, Today raises a fresh card.
		d.Status = executive.StatusResolved
		d.SnoozeUntil = nil
		d.UpdatedAt = now
		if err := s.store.UpdateDecision(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Act(ctx context.Context, ac actor.Context, cmd Command) (View, error) {
	if ac.Scope != staffscope.GeneralManager && ac.Scope != staffscope.DepartmentManager {
		return View{}, errx.Forbidden("forbidden", "decisions are a manager surface")
	}
	if !rbac.Allows(ac.Scope, rbac.ViewDashboard) {
		return View{}, errx.Forbidden("forbidden", "cannot act on the brief")
	}
	action, err := executive.ParseAction(cmd.Action)
	if err != nil {
		return View{}, err
	}
	if action == executive.ActionAssign {
		if !rbac.Allows(ac.Scope, rbac.ManageTasks) {
			return View{}, errx.Forbidden("forbidden", "managers assign follow-up work")
		}
		if strings.TrimSpace(cmd.AssigneeUserID) == "" {
			return View{}, errx.Invalid("invalid_assignee", "assignee is required")
		}
	}
	now := s.clock.Now()
	creator, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	d, created, err := s.loadOrOpen(ctx, ac.Workspace.ID, creator, cmd, now)
	if err != nil {
		return View{}, err
	}
	if d.WorkspaceID != ac.Workspace.ID {
		return View{}, errx.NotFound("not_found", "decision not found")
	}
	d.Fingerprint = firstNonEmpty(cmd.Fingerprint, d.Fingerprint)
	d.Kind = firstNonEmpty(cmd.Kind, d.Kind)
	d.TitleKey = firstNonEmpty(cmd.TitleKey, d.TitleKey)
	d.Detail = firstNonEmpty(cmd.Detail, d.Detail)
	d.Severity = firstNonEmpty(cmd.Severity, d.Severity)
	d.TargetTab = firstNonEmpty(cmd.TargetTab, d.TargetTab)

	snooze := executive.EndOfLocalDay(now, ac.Workspace.Timezone)
	if err := d.Apply(action, now, snooze); err != nil {
		return View{}, err
	}
	if action == executive.ActionAssign {
		title := strings.TrimSpace(cmd.Title)
		if title == "" {
			title = "Follow up: late staff work"
		}
		start := now
		end := now.Add(2 * time.Hour)
		taskID, err := s.assign.Assign(ctx, ac, title, cmd.Description, cmd.AssigneeUserID, start, end)
		if err != nil {
			return View{}, err
		}
		assignee, err := id.Parse(cmd.AssigneeUserID)
		if err != nil {
			return View{}, errx.Invalid("invalid_assignee", "invalid assignee")
		}
		d.LinkedTaskID = taskID
		d.AssigneeUserID = assignee
	}
	if created {
		if err := s.store.CreateDecision(ctx, d); err != nil {
			return View{}, err
		}
	} else if err := s.store.UpdateDecision(ctx, d); err != nil {
		return View{}, err
	}
	return toView(d), nil
}

func (s *Service) loadOrOpen(ctx context.Context, workspaceID, creator id.ID, cmd Command, now time.Time) (executive.Decision, bool, error) {
	list, err := s.store.ListDecisions(ctx, workspaceID)
	if err != nil {
		return executive.Decision{}, false, err
	}
	for _, existing := range list {
		if existing.SourceKey == cmd.SourceKey && existing.Active() {
			return existing, false, nil
		}
	}
	d, err := executive.NewDecision(
		workspaceID, creator, cmd.SourceKey, cmd.Fingerprint, cmd.Kind,
		cmd.TitleKey, cmd.Detail, cmd.Severity, cmd.TargetTab, now,
	)
	return d, true, err
}

func firstNonEmpty(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}
