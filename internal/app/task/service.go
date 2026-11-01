package task

import (
	"context"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	store   Repository
	members MembershipReader
	clock   clock.Clock
}

func New(store Repository, members MembershipReader, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{store: store, members: members, clock: clk}
}

type View struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	AssigneeUserID string  `json:"assigneeUserId"`
	ScheduledStart string  `json:"scheduledStartAt"`
	ScheduledEnd   string  `json:"scheduledEndAt"`
	Status         string  `json:"status"`
	IsOverdue      bool    `json:"isOverdue"`
	CompletedAt    *string `json:"completedAt"`
}

func toView(t work.Task, now time.Time) View {
	v := View{
		ID:             t.ID.String(),
		Title:          t.Title,
		Description:    t.Description,
		AssigneeUserID: t.AssigneeUserID.String(),
		ScheduledStart: t.ScheduledStart.Format(time.RFC3339),
		ScheduledEnd:   t.ScheduledEnd.Format(time.RFC3339),
		Status:         t.Status,
		IsOverdue:      t.Overdue(now),
	}
	if t.CompletedAt != nil {
		s := t.CompletedAt.Format(time.RFC3339)
		v.CompletedAt = &s
	}
	return v
}

func (s *Service) List(ctx context.Context, ac actor.Context) ([]View, error) {
	if !rbac.Allows(ac.Scope, rbac.ViewTasks) {
		return nil, errx.Forbidden("forbidden", "managers view department tasks")
	}
	all, err := s.store.ListTasksByWorkspace(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	dept := staffscope.DepartmentScope(ac.Actor, ac.Catalog)
	now := s.clock.Now()
	out := make([]View, 0, len(all))
	for _, t := range all {
		if t.WorkspaceID != ac.Workspace.ID {
			continue
		}
		if dept != "" {
			m, err := s.membershipForUser(ctx, ac.Workspace.ID, t.AssigneeUserID)
			if err != nil || m.Department != dept {
				continue
			}
		}
		out = append(out, toView(t, now))
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, ac actor.Context, title, description, assigneeUserID string, start, end time.Time) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.ManageTasks) {
		return View{}, errx.Forbidden("forbidden", "managers assign tasks")
	}
	assignee, err := id.Parse(assigneeUserID)
	if err != nil {
		return View{}, errx.Invalid("invalid_assignee", "invalid assignee")
	}
	m, err := s.membershipForUser(ctx, ac.Workspace.ID, assignee)
	if err != nil {
		return View{}, errx.NotFound("assignee_not_found", "assignee is not in this workspace")
	}
	if m.Role != identity.RolePersonnel {
		return View{}, errx.Invalid("invalid_assignee", "tasks are assigned to personnel")
	}
	if !staffscope.CanManageStaffMember(ac.Actor, staffscope.FromMembership(m), ac.Catalog) {
		return View{}, errx.Forbidden("forbidden", "cannot assign outside your department")
	}
	creator, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	task, err := work.NewTask(ac.Workspace.ID, assignee, creator, title, description, start, end, s.clock.Now())
	if err != nil {
		return View{}, err
	}
	if err := s.store.CreateTask(ctx, task); err != nil {
		return View{}, err
	}
	return toView(task, s.clock.Now()), nil
}

func (s *Service) PortalTasks(ctx context.Context, ac actor.Context) ([]View, error) {
	if ac.Scope != staffscope.StaffPersonnel {
		return nil, errx.Forbidden("forbidden", "staff portal is for employees")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return nil, errx.Unauthenticated("invalid token")
	}
	list, err := s.store.ListTasksByAssignee(ctx, ac.Workspace.ID, uid)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	out := make([]View, 0, len(list))
	for _, t := range list {
		if t.WorkspaceID != ac.Workspace.ID {
			continue
		}
		out = append(out, toView(t, now))
	}
	return out, nil
}

func (s *Service) Complete(ctx context.Context, ac actor.Context, taskID string) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.ManageTasks) {
		return View{}, errx.Forbidden("forbidden", "managers complete tasks")
	}
	tid, err := id.Parse(taskID)
	if err != nil {
		return View{}, errx.Invalid("invalid_id", "invalid task id")
	}
	task, err := s.store.GetTask(ctx, tid)
	if err != nil {
		return View{}, err
	}
	if task.WorkspaceID != ac.Workspace.ID {
		return View{}, errx.NotFound("not_found", "task not found")
	}
	m, err := s.membershipForUser(ctx, ac.Workspace.ID, task.AssigneeUserID)
	if err != nil {
		return View{}, errx.NotFound("not_found", "task not found")
	}
	if !staffscope.CanManageStaffMember(ac.Actor, staffscope.FromMembership(m), ac.Catalog) {
		return View{}, errx.Forbidden("forbidden", "cannot complete outside your department")
	}
	if err := task.Complete(s.clock.Now()); err != nil {
		return View{}, err
	}
	if err := s.store.UpdateTask(ctx, task); err != nil {
		return View{}, err
	}
	return toView(task, s.clock.Now()), nil
}

func (s *Service) CompletePortal(ctx context.Context, ac actor.Context, taskID string) (View, error) {
	if ac.Scope != staffscope.StaffPersonnel {
		return View{}, errx.Forbidden("forbidden", "staff portal is for employees")
	}
	tid, err := id.Parse(taskID)
	if err != nil {
		return View{}, errx.Invalid("invalid_id", "invalid task id")
	}
	task, err := s.store.GetTask(ctx, tid)
	if err != nil {
		return View{}, err
	}
	if task.WorkspaceID != ac.Workspace.ID || task.AssigneeUserID.String() != ac.Principal.UserID {
		return View{}, errx.NotFound("not_found", "task not found")
	}
	if err := task.Complete(s.clock.Now()); err != nil {
		return View{}, err
	}
	if err := s.store.UpdateTask(ctx, task); err != nil {
		return View{}, err
	}
	return toView(task, s.clock.Now()), nil
}

func (s *Service) CreateRoutine(ctx context.Context, ac actor.Context, title, description, assigneeUserID, start, end string) (work.Routine, error) {
	if !rbac.Allows(ac.Scope, rbac.ManageTasks) {
		return work.Routine{}, errx.Forbidden("forbidden", "managers create routines")
	}
	assignee, err := id.Parse(assigneeUserID)
	if err != nil {
		return work.Routine{}, errx.Invalid("invalid_assignee", "invalid assignee")
	}
	m, err := s.membershipForUser(ctx, ac.Workspace.ID, assignee)
	if err != nil {
		return work.Routine{}, errx.NotFound("assignee_not_found", "assignee is not in this workspace")
	}
	if !staffscope.CanManageStaffMember(ac.Actor, staffscope.FromMembership(m), ac.Catalog) {
		return work.Routine{}, errx.Forbidden("forbidden", "cannot assign outside your department")
	}
	creator, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return work.Routine{}, errx.Unauthenticated("invalid token")
	}
	r, err := work.NewRoutine(ac.Workspace.ID, creator, assignee, title, description, start, end, s.clock.Now())
	if err != nil {
		return work.Routine{}, err
	}
	if err := s.store.CreateRoutine(ctx, r); err != nil {
		return work.Routine{}, err
	}
	return r, nil
}

func (s *Service) ListRoutines(ctx context.Context, ac actor.Context) ([]work.Routine, error) {
	if !rbac.Allows(ac.Scope, rbac.ViewTasks) {
		return nil, errx.Forbidden("forbidden", "managers view routines")
	}
	list, err := s.store.ListRoutinesByWorkspace(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	dept := staffscope.DepartmentScope(ac.Actor, ac.Catalog)
	if dept == "" {
		return list, nil
	}
	out := make([]work.Routine, 0)
	for _, r := range list {
		m, err := s.membershipForUser(ctx, ac.Workspace.ID, r.AssigneeUserID)
		if err != nil || m.Department != dept {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Service) membershipForUser(ctx context.Context, workspaceID, userID id.ID) (identity.Membership, error) {
	all, err := s.members.ListWorkspaceMemberships(ctx, workspaceID)
	if err != nil {
		return identity.Membership{}, err
	}
	for _, m := range all {
		if m.UserID == userID && m.WorkspaceID == workspaceID {
			return m, nil
		}
	}
	return identity.Membership{}, errx.NotFound("not_found", "membership not found")
}
