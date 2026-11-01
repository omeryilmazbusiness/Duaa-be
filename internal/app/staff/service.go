package staff

import (
	"context"
	"strings"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	users     UserWriter
	members   MembershipRepo
	passwords PasswordHasher
	clock     clock.Clock
}

func New(users UserWriter, members MembershipRepo, passwords PasswordHasher, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{users: users, members: members, passwords: passwords, clock: clk}
}

type MemberView struct {
	ID              string `json:"id"`
	UserID          string `json:"userId"`
	Email           string `json:"email"`
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	Role            string `json:"role"`
	StaffDepartment string `json:"staffDepartment"`
	EmployeeNumber  string `json:"employeeNumber,omitempty"`
	IsActive        bool   `json:"isActive"`
}

func (s *Service) ListPersonnel(ctx context.Context, ac actor.Context) ([]MemberView, error) {
	if !rbac.Allows(ac.Scope, rbac.ManageStaff) && !rbac.Allows(ac.Scope, rbac.ViewReviews) {
		return nil, errx.Forbidden("forbidden", "staff management requires a manager")
	}
	list, err := s.members.ListWorkspaceMemberships(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	dept := staffscope.DepartmentScope(ac.Actor, ac.Catalog)
	out := make([]MemberView, 0)
	for _, m := range list {
		if m.Role != identity.RolePersonnel {
			continue
		}
		if dept != "" && m.Department != dept {
			continue
		}
		view, err := s.view(ctx, m)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) CreatePersonnel(ctx context.Context, ac actor.Context, first, last, department, employeeNumber, email, password string) (MemberView, error) {
	if !rbac.Allows(ac.Scope, rbac.ManageStaff) {
		return MemberView{}, errx.Forbidden("forbidden", "staff management requires a manager")
	}
	dept := strings.ToUpper(strings.TrimSpace(department))
	if scoped := staffscope.DepartmentScope(ac.Actor, ac.Catalog); scoped != "" {
		dept = scoped
	}
	if !tenancy.ContainsDept(ac.Catalog.Departments(), tenancy.Department(dept)) {
		return MemberView{}, errx.Invalid("invalid_department", "department is not valid for this workspace")
	}
	memberActor := staffscope.Actor{Role: identity.RolePersonnel, Department: dept}
	if !staffscope.CanManageStaffMember(ac.Actor, memberActor, ac.Catalog) {
		return MemberView{}, errx.Forbidden("forbidden", "cannot create staff outside your department")
	}

	now := s.clock.Now()
	user := identity.User{
		ID:        id.New(),
		FirstName: strings.TrimSpace(first),
		LastName:  strings.TrimSpace(last),
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m := identity.Membership{
		ID:          id.New(),
		WorkspaceID: ac.Workspace.ID,
		UserID:      user.ID,
		Role:        identity.RolePersonnel,
		Department:  dept,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if tenancy.ContainsDept(ac.Catalog.EmployeeNumberDepartments(), tenancy.Department(dept)) {
		number, err := identity.NormalizeEmployeeNumber(employeeNumber)
		if err != nil {
			return MemberView{}, err
		}
		m.EmployeeNumber = number
		user.Email = identity.PlaceholderEmail(ac.Workspace.ID, number)
	} else {
		normalized, err := identity.NormalizeEmail(email)
		if err != nil {
			return MemberView{}, err
		}
		if len(password) < 8 {
			return MemberView{}, errx.Invalid("invalid_password", "password must be at least 8 characters")
		}
		hash, err := s.passwords.Hash(password)
		if err != nil {
			return MemberView{}, err
		}
		user.Email = normalized
		user.PasswordHash = hash
	}
	if user.FirstName == "" || user.LastName == "" {
		return MemberView{}, errx.Invalid("invalid_name", "first and last name are required")
	}
	if err := s.users.CreateUser(ctx, user); err != nil {
		return MemberView{}, err
	}
	if err := s.members.CreateMembership(ctx, m); err != nil {
		return MemberView{}, err
	}
	return s.view(ctx, m)
}

func (s *Service) ListDepartmentManagers(ctx context.Context, ac actor.Context) ([]MemberView, error) {
	if ac.Scope != staffscope.GeneralManager {
		return nil, errx.Forbidden("forbidden", "only the general manager can list department managers")
	}
	list, err := s.members.ListWorkspaceMemberships(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberView, 0)
	for _, m := range list {
		if m.Role != identity.RoleManager || m.Department == "" {
			continue
		}
		if !tenancy.ContainsDept(ac.Catalog.DepartmentManagerDepartments(), tenancy.Department(m.Department)) {
			continue
		}
		view, err := s.view(ctx, m)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) CreateDepartmentManager(ctx context.Context, ac actor.Context, first, last, email, password, department string) (MemberView, error) {
	if ac.Scope != staffscope.GeneralManager {
		return MemberView{}, errx.Forbidden("forbidden", "only the general manager can create department managers")
	}
	dept := strings.ToUpper(strings.TrimSpace(department))
	if !tenancy.ContainsDept(ac.Catalog.DepartmentManagerDepartments(), tenancy.Department(dept)) {
		return MemberView{}, errx.Invalid("invalid_department", "department cannot have a manager")
	}
	if _, err := s.members.GetActiveDepartmentManager(ctx, ac.Workspace.ID, dept); err == nil {
		return MemberView{}, errx.Conflict("dept_manager_exists", "an active department manager already exists for this department")
	}
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return MemberView{}, err
	}
	if len(password) < 8 {
		return MemberView{}, errx.Invalid("invalid_password", "password must be at least 8 characters")
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return MemberView{}, err
	}
	now := s.clock.Now()
	user := identity.User{
		ID:           id.New(),
		Email:        normalized,
		PasswordHash: hash,
		FirstName:    strings.TrimSpace(first),
		LastName:     strings.TrimSpace(last),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if user.FirstName == "" || user.LastName == "" {
		return MemberView{}, errx.Invalid("invalid_name", "first and last name are required")
	}
	m := identity.Membership{
		ID:          id.New(),
		WorkspaceID: ac.Workspace.ID,
		UserID:      user.ID,
		Role:        identity.RoleManager,
		Department:  dept,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.users.CreateUser(ctx, user); err != nil {
		return MemberView{}, err
	}
	if err := s.members.CreateMembership(ctx, m); err != nil {
		return MemberView{}, err
	}
	return s.view(ctx, m)
}

func (s *Service) Deactivate(ctx context.Context, ac actor.Context, membershipID string) error {
	if !rbac.Allows(ac.Scope, rbac.ManageStaff) {
		return errx.Forbidden("forbidden", "staff management requires a manager")
	}
	mid, err := id.Parse(membershipID)
	if err != nil {
		return errx.Invalid("invalid_id", "invalid membership id")
	}
	m, err := s.members.GetMembership(ctx, mid)
	if err != nil {
		return err
	}
	if m.WorkspaceID != ac.Workspace.ID {
		return errx.NotFound("not_found", "staff member not found")
	}
	target := staffscope.FromMembership(m)
	if ac.Scope == staffscope.GeneralManager && m.Role == identity.RoleManager && m.Department != "" {
		m.IsActive = false
		m.UpdatedAt = s.clock.Now()
		return s.members.UpdateMembership(ctx, m)
	}
	if !staffscope.CanManageStaffMember(ac.Actor, target, ac.Catalog) {
		return errx.Forbidden("forbidden", "cannot manage this staff member")
	}
	m.IsActive = false
	m.UpdatedAt = s.clock.Now()
	return s.members.UpdateMembership(ctx, m)
}

func (s *Service) view(ctx context.Context, m identity.Membership) (MemberView, error) {
	user, err := s.users.GetUser(ctx, m.UserID)
	if err != nil {
		return MemberView{}, err
	}
	email := user.Email
	if strings.HasSuffix(email, "@staff.internal") {
		email = ""
	}
	return MemberView{
		ID:              m.ID.String(),
		UserID:          user.ID.String(),
		Email:           email,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Role:            m.Role,
		StaffDepartment: m.Department,
		EmployeeNumber:  m.EmployeeNumber,
		IsActive:        m.IsActive && user.IsActive,
	}, nil
}

