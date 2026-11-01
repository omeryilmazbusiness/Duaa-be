package memory

import (
	"context"
	"sync"

	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// Store is a thread-safe in-memory adapter used by tests and local boots without Postgres.
type Store struct {
	mu              sync.RWMutex
	orgs            map[id.ID]tenancy.Organization
	workspaces      map[id.ID]tenancy.Workspace
	users           map[id.ID]identity.User
	memberships     map[id.ID]identity.Membership
	tasks           map[id.ID]work.Task
	routines        map[id.ID]work.Routine
	decisions       map[id.ID]executive.Decision
	reports         map[id.ID]opsreport.Report
	closes          map[string]finance.Close
	approvals       map[id.ID]finance.Approval
	approvalPersist string
	aiSettings      map[id.ID]assistant.Settings
	aiUsage         map[string]int
	aiPersist       string
	cafeDefs        map[id.ID]cafe.Definition
	cafePersist     string
	qrCodes         map[id.ID]cafe.Code
	qrByToken       map[string]id.ID
	voices          map[id.ID]cafe.Voice
	recoveries      map[id.ID]cafe.Recovery
	qrPersist       string
	intakes         map[string]cafe.Intake
	intakePersist   string
	handovers       map[id.ID]cafe.Handover
	handoverPersist string
	checkTemplates  map[id.ID]cafe.ChecklistTemplate
	checkRuns       map[string]cafe.ChecklistRun
	checkPersist    string
	emailIndex      map[string]id.ID
	slugIndex       map[string]id.ID
	employeeIdx     map[string]id.ID
}

func New() *Store {
	return &Store{
		orgs:           map[id.ID]tenancy.Organization{},
		workspaces:     map[id.ID]tenancy.Workspace{},
		users:          map[id.ID]identity.User{},
		memberships:    map[id.ID]identity.Membership{},
		tasks:          map[id.ID]work.Task{},
		routines:       map[id.ID]work.Routine{},
		decisions:      map[id.ID]executive.Decision{},
		reports:        map[id.ID]opsreport.Report{},
		closes:         map[string]finance.Close{},
		approvals:      map[id.ID]finance.Approval{},
		aiSettings:     map[id.ID]assistant.Settings{},
		aiUsage:        map[string]int{},
		cafeDefs:       map[id.ID]cafe.Definition{},
		qrCodes:        map[id.ID]cafe.Code{},
		qrByToken:      map[string]id.ID{},
		voices:         map[id.ID]cafe.Voice{},
		recoveries:     map[id.ID]cafe.Recovery{},
		intakes:        map[string]cafe.Intake{},
		handovers:      map[id.ID]cafe.Handover{},
		checkTemplates: map[id.ID]cafe.ChecklistTemplate{},
		checkRuns:      map[string]cafe.ChecklistRun{},
		emailIndex:     map[string]id.ID{},
		slugIndex:      map[string]id.ID{},
		employeeIdx:    map[string]id.ID{},
	}
}

func employeeKey(workspaceID id.ID, number string) string {
	return workspaceID.String() + ":" + number
}

func (s *Store) CreateOrganization(_ context.Context, o tenancy.Organization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orgs[o.ID] = o
	return nil
}

func (s *Store) CreateWorkspace(_ context.Context, w tenancy.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.slugIndex[w.Slug]; ok {
		return errx.Conflict("slug_taken", "workspace slug already exists")
	}
	s.workspaces[w.ID] = w
	s.slugIndex[w.Slug] = w.ID
	return nil
}

func (s *Store) GetWorkspace(_ context.Context, wid id.ID) (tenancy.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.workspaces[wid]
	if !ok {
		return tenancy.Workspace{}, errx.NotFound("workspace_not_found", "workspace not found")
	}
	return w, nil
}

func (s *Store) GetWorkspaceBySlug(_ context.Context, slug string) (tenancy.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wid, ok := s.slugIndex[slug]
	if !ok {
		return tenancy.Workspace{}, errx.NotFound("workspace_not_found", "workspace not found")
	}
	return s.workspaces[wid], nil
}

func (s *Store) UpdateWorkspace(_ context.Context, w tenancy.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.workspaces[w.ID]
	if !ok {
		return errx.NotFound("workspace_not_found", "workspace not found")
	}
	cur.Name = w.Name
	cur.Timezone = w.Timezone
	cur.UpdatedAt = w.UpdatedAt
	s.workspaces[w.ID] = cur
	return nil
}

func (s *Store) CreateUser(_ context.Context, u identity.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.Email != "" {
		if _, exists := s.emailIndex[u.Email]; exists {
			return errx.Conflict("email_taken", "a user with this email address already exists")
		}
		s.emailIndex[u.Email] = u.ID
	}
	s.users[u.ID] = u
	return nil
}

func (s *Store) GetUser(_ context.Context, uid id.ID) (identity.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[uid]
	if !ok {
		return identity.User{}, errx.NotFound("user_not_found", "user not found")
	}
	return u, nil
}

func (s *Store) GetUserByEmail(_ context.Context, email string) (identity.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uid, ok := s.emailIndex[email]
	if !ok {
		return identity.User{}, errx.NotFound("user_not_found", "user not found")
	}
	return s.users[uid], nil
}

func (s *Store) UpdateUser(_ context.Context, u identity.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.ID]; !ok {
		return errx.NotFound("user_not_found", "user not found")
	}
	s.users[u.ID] = u
	return nil
}

func (s *Store) CreateMembership(_ context.Context, m identity.Membership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.memberships {
		if existing.WorkspaceID == m.WorkspaceID && existing.UserID == m.UserID {
			return errx.Conflict("membership_exists", "user already belongs to this workspace")
		}
	}
	if m.EmployeeNumber != "" {
		key := employeeKey(m.WorkspaceID, m.EmployeeNumber)
		if _, ok := s.employeeIdx[key]; ok {
			return errx.Conflict("employee_number_taken", "employee number already in use")
		}
		s.employeeIdx[key] = m.ID
	}
	s.memberships[m.ID] = m
	return nil
}

func (s *Store) GetMembership(_ context.Context, mid id.ID) (identity.Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.memberships[mid]
	if !ok {
		return identity.Membership{}, errx.NotFound("membership_not_found", "membership not found")
	}
	return m, nil
}

func (s *Store) ListMembershipsByUser(_ context.Context, userID id.ID) ([]identity.Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]identity.Membership, 0)
	for _, m := range s.memberships {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Store) ListWorkspaceMemberships(_ context.Context, workspaceID id.ID) ([]identity.Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]identity.Membership, 0)
	for _, m := range s.memberships {
		if m.WorkspaceID == workspaceID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Store) UpdateMembership(_ context.Context, m identity.Membership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.memberships[m.ID]; !ok {
		return errx.NotFound("membership_not_found", "membership not found")
	}
	s.memberships[m.ID] = m
	return nil
}

func (s *Store) GetMembershipByEmployee(_ context.Context, workspaceID id.ID, employeeNumber string) (identity.Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mid, ok := s.employeeIdx[employeeKey(workspaceID, employeeNumber)]
	if !ok {
		return identity.Membership{}, errx.NotFound("membership_not_found", "membership not found")
	}
	return s.memberships[mid], nil
}

func (s *Store) GetActiveDepartmentManager(_ context.Context, workspaceID id.ID, department string) (identity.Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.memberships {
		if m.WorkspaceID == workspaceID && m.Role == identity.RoleManager && m.Department == department && m.IsActive {
			return m, nil
		}
	}
	return identity.Membership{}, errx.NotFound("not_found", "no active department manager")
}

func (s *Store) CreateTask(_ context.Context, t work.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
	return nil
}

func (s *Store) GetTask(_ context.Context, tid id.ID) (work.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[tid]
	if !ok {
		return work.Task{}, errx.NotFound("task_not_found", "task not found")
	}
	return t, nil
}

func (s *Store) ListTasksByWorkspace(_ context.Context, workspaceID id.ID) ([]work.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]work.Task, 0)
	for _, t := range s.tasks {
		if t.WorkspaceID == workspaceID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *Store) ListTasksByAssignee(_ context.Context, workspaceID, assignee id.ID) ([]work.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]work.Task, 0)
	for _, t := range s.tasks {
		if t.WorkspaceID == workspaceID && t.AssigneeUserID == assignee {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *Store) UpdateTask(_ context.Context, t work.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[t.ID]; !ok {
		return errx.NotFound("task_not_found", "task not found")
	}
	s.tasks[t.ID] = t
	return nil
}

func (s *Store) CreateRoutine(_ context.Context, r work.Routine) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routines[r.ID] = r
	return nil
}

func (s *Store) ListRoutinesByWorkspace(_ context.Context, workspaceID id.ID) ([]work.Routine, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]work.Routine, 0)
	for _, r := range s.routines {
		if r.WorkspaceID == workspaceID {
			out = append(out, r)
		}
	}
	return out, nil
}
