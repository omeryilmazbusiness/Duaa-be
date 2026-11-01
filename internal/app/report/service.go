package report

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	store   Repository
	members MembershipReader
	users   UserReader
	assign  AssignmentPort
	clock   clock.Clock
}

func New(store Repository, members MembershipReader, users UserReader, assign AssignmentPort, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{store: store, members: members, users: users, assign: assign, clock: clk}
}

type View struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind"`
	Title              string `json:"title"`
	Detail             string `json:"detail"`
	Status             string `json:"status"`
	ReporterUserID     string `json:"reporterUserId"`
	ReporterName       string `json:"reporterName"`
	ReporterDepartment string `json:"reporterDepartment,omitempty"`
	AssigneeUserID     string `json:"assigneeUserId,omitempty"`
	LinkedTaskID       string `json:"linkedTaskId,omitempty"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

func (s *Service) Submit(ctx context.Context, ac actor.Context, kind, title, detail string) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.SubmitReport) {
		return View{}, errx.Forbidden("forbidden", "staff submit floor reports")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	r, err := opsreport.New(ac.Workspace.ID, uid, kind, title, detail, s.clock.Now())
	if err != nil {
		return View{}, err
	}
	if err := s.store.CreateReport(ctx, r); err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac.Workspace.ID, r)
}

func (s *Service) List(ctx context.Context, ac actor.Context) ([]View, error) {
	inbox := rbac.Allows(ac.Scope, rbac.ViewReports)
	if !inbox && !rbac.Allows(ac.Scope, rbac.SubmitReport) {
		return nil, errx.Forbidden("forbidden", "cannot list reports")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return nil, errx.Unauthenticated("invalid token")
	}
	all, err := s.store.ListReports(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(all))
	for _, r := range all {
		if r.WorkspaceID != ac.Workspace.ID {
			continue
		}
		if !inbox {
			if r.ReporterUserID != uid {
				continue
			}
		} else if ac.Scope == staffscope.DepartmentManager {
			if !s.reporterInDepartment(ctx, ac.Workspace.ID, r.ReporterUserID, ac.Actor.Department) {
				continue
			}
		}
		view, err := s.toView(ctx, ac.Workspace.ID, r)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func (s *Service) Act(ctx context.Context, ac actor.Context, reportID, action, assigneeUserID, title string) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.ViewReports) {
		return View{}, errx.Forbidden("forbidden", "managers act on reports")
	}
	parsed, err := opsreport.ParseAction(action)
	if err != nil {
		return View{}, err
	}
	if parsed == opsreport.ActionAssign && !rbac.Allows(ac.Scope, rbac.ManageTasks) {
		return View{}, errx.Forbidden("forbidden", "managers assign follow-up work")
	}
	rid, err := id.Parse(reportID)
	if err != nil {
		return View{}, errx.Invalid("invalid_id", "invalid report id")
	}
	r, err := s.store.GetReport(ctx, rid)
	if err != nil {
		return View{}, err
	}
	if r.WorkspaceID != ac.Workspace.ID {
		return View{}, errx.NotFound("not_found", "report not found")
	}
	now := s.clock.Now()
	switch parsed {
	case opsreport.ActionAssign:
		if strings.TrimSpace(assigneeUserID) == "" {
			return View{}, errx.Invalid("invalid_assignee", "assignee is required")
		}
		taskTitle := strings.TrimSpace(title)
		if taskTitle == "" {
			taskTitle = "Follow up: " + r.Title
		}
		taskID, err := s.assign.Assign(ctx, ac, taskTitle, r.Detail, assigneeUserID, now, now.Add(2*time.Hour))
		if err != nil {
			return View{}, err
		}
		assignee, err := id.Parse(assigneeUserID)
		if err != nil {
			return View{}, errx.Invalid("invalid_assignee", "invalid assignee")
		}
		if err := r.Assign(assignee, taskID, now); err != nil {
			return View{}, err
		}
	case opsreport.ActionResolve:
		if err := r.Resolve(now); err != nil {
			return View{}, err
		}
	}
	if err := s.store.UpdateReport(ctx, r); err != nil {
		return View{}, err
	}
	return s.toView(ctx, ac.Workspace.ID, r)
}

func (s *Service) reporterInDepartment(ctx context.Context, workspaceID, reporter id.ID, department string) bool {
	list, err := s.members.ListWorkspaceMemberships(ctx, workspaceID)
	if err != nil {
		return false
	}
	for _, m := range list {
		if m.UserID == reporter && m.IsActive {
			return m.Department == department
		}
	}
	return false
}

func (s *Service) toView(ctx context.Context, workspaceID id.ID, r opsreport.Report) (View, error) {
	v := View{
		ID:             r.ID.String(),
		Kind:           r.Kind,
		Title:          r.Title,
		Detail:         r.Detail,
		Status:         r.Status,
		ReporterUserID: r.ReporterUserID.String(),
		CreatedAt:      r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      r.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if r.AssigneeUserID != id.Zero() {
		v.AssigneeUserID = r.AssigneeUserID.String()
	}
	if r.LinkedTaskID != id.Zero() {
		v.LinkedTaskID = r.LinkedTaskID.String()
	}
	if u, err := s.users.GetUser(ctx, r.ReporterUserID); err == nil {
		v.ReporterName = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}
	if list, err := s.members.ListWorkspaceMemberships(ctx, workspaceID); err == nil {
		for _, m := range list {
			if m.UserID == r.ReporterUserID {
				v.ReporterDepartment = m.Department
				break
			}
		}
	}
	return v, nil
}
