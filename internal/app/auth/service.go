package auth

import (
	"context"
	"strings"

	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	users    UserReader
	members  MembershipReader
	spaces   WorkspaceReader
	registry *tenancy.Registry
	tokens   Tokens
	passwords PasswordChecker
	clock    clock.Clock
}

func New(users UserReader, members MembershipReader, spaces WorkspaceReader, registry *tenancy.Registry, tokens Tokens, passwords PasswordChecker, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{users: users, members: members, spaces: spaces, registry: registry, tokens: tokens, passwords: passwords, clock: clk}
}

type LoginResult struct {
	Token      string              `json:"token,omitempty"`
	ExpiresIn  int                 `json:"expiresIn,omitempty"`
	Workspaces []WorkspaceOption   `json:"workspaces,omitempty"`
	NeedsPick  bool                `json:"needsWorkspacePick"`
	Session    *SessionPayload     `json:"session,omitempty"`
}

type WorkspaceOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Slug string `json:"slug"`
}

type SessionPayload struct {
	User       UserPayload       `json:"user"`
	Membership MembershipPayload `json:"membership"`
	Workspace  WorkspacePayload  `json:"workspace"`
	Scope      string            `json:"scope"`
	ScopeLabel string            `json:"scopeLabel"`
	Tabs       []string          `json:"tabs"`
	DefaultTab string            `json:"defaultTab"`
	Surface    string            `json:"surface"`
}

type UserPayload struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type MembershipPayload struct {
	ID             string `json:"id"`
	Role           string `json:"role"`
	StaffDepartment string `json:"staffDepartment"`
	EmployeeNumber string `json:"employeeNumber,omitempty"`
}

type WorkspacePayload struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Slug string `json:"slug"`
}

func (s *Service) LoginEmail(ctx context.Context, email, password string) (LoginResult, error) {
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return LoginResult{}, err
	}
	user, err := s.users.GetUserByEmail(ctx, normalized)
	if err != nil {
		return LoginResult{}, errx.Unauthenticated("invalid credentials")
	}
	if !user.IsActive || !s.passwords.Check(user.PasswordHash, password) {
		return LoginResult{}, errx.Unauthenticated("invalid credentials")
	}
	memberships, err := s.activeMemberships(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if len(memberships) == 0 {
		return LoginResult{}, errx.Forbidden("no_membership", "no active workspace membership")
	}
	if len(memberships) > 1 {
		opts, err := s.options(ctx, memberships)
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{NeedsPick: true, Workspaces: opts}, nil
	}
	return s.issue(ctx, user, memberships[0])
}

func (s *Service) SelectWorkspace(ctx context.Context, email, password, workspaceID string) (LoginResult, error) {
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return LoginResult{}, err
	}
	user, err := s.users.GetUserByEmail(ctx, normalized)
	if err != nil {
		return LoginResult{}, errx.Unauthenticated("invalid credentials")
	}
	if !user.IsActive || !s.passwords.Check(user.PasswordHash, password) {
		return LoginResult{}, errx.Unauthenticated("invalid credentials")
	}
	wid, err := id.Parse(workspaceID)
	if err != nil {
		return LoginResult{}, errx.Invalid("invalid_workspace", "invalid workspace id")
	}
	memberships, err := s.activeMemberships(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	for _, m := range memberships {
		if m.WorkspaceID == wid {
			return s.issue(ctx, user, m)
		}
	}
	return LoginResult{}, errx.Forbidden("workspace_forbidden", "not a member of this workspace")
}

func (s *Service) LoginEmployee(ctx context.Context, workspaceSlug, employeeNumber string) (LoginResult, error) {
	slug, err := tenancy.NormalizeSlug(workspaceSlug)
	if err != nil {
		return LoginResult{}, err
	}
	number, err := identity.NormalizeEmployeeNumber(employeeNumber)
	if err != nil {
		return LoginResult{}, err
	}
	ws, err := s.spaces.GetWorkspaceBySlug(ctx, slug)
	if err != nil {
		return LoginResult{}, errx.Unauthenticated("invalid credentials")
	}
	m, err := s.members.GetMembershipByEmployee(ctx, ws.ID, number)
	if err != nil || !m.IsActive {
		return LoginResult{}, errx.Unauthenticated("invalid credentials")
	}
	user, err := s.users.GetUser(ctx, m.UserID)
	if err != nil || !user.IsActive {
		return LoginResult{}, errx.Unauthenticated("invalid credentials")
	}
	return s.issue(ctx, user, m)
}

func (s *Service) SessionFromPrincipal(ctx context.Context, p ctxkey.Principal) (SessionPayload, error) {
	uid, err := id.Parse(p.UserID)
	if err != nil {
		return SessionPayload{}, errx.Unauthenticated("invalid token")
	}
	mid, err := id.Parse(p.MembershipID)
	if err != nil {
		return SessionPayload{}, errx.Unauthenticated("invalid token")
	}
	user, err := s.users.GetUser(ctx, uid)
	if err != nil {
		return SessionPayload{}, err
	}
	m, err := s.members.GetMembership(ctx, mid)
	if err != nil {
		return SessionPayload{}, err
	}
	if m.UserID != user.ID || !m.IsActive {
		return SessionPayload{}, errx.Forbidden("membership_inactive", "membership is not active")
	}
	return s.session(ctx, user, m)
}

func (s *Service) SwitchWorkspace(ctx context.Context, p ctxkey.Principal, workspaceID string) (LoginResult, error) {
	uid, err := id.Parse(p.UserID)
	if err != nil {
		return LoginResult{}, errx.Unauthenticated("invalid token")
	}
	wid, err := id.Parse(workspaceID)
	if err != nil {
		return LoginResult{}, errx.Invalid("invalid_workspace", "invalid workspace id")
	}
	user, err := s.users.GetUser(ctx, uid)
	if err != nil {
		return LoginResult{}, err
	}
	memberships, err := s.activeMemberships(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	for _, m := range memberships {
		if m.WorkspaceID == wid {
			return s.issue(ctx, user, m)
		}
	}
	return LoginResult{}, errx.Forbidden("workspace_forbidden", "not a member of this workspace")
}

func (s *Service) activeMemberships(ctx context.Context, userID id.ID) ([]identity.Membership, error) {
	all, err := s.members.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]identity.Membership, 0, len(all))
	for _, m := range all {
		if m.IsActive {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Service) options(ctx context.Context, memberships []identity.Membership) ([]WorkspaceOption, error) {
	out := make([]WorkspaceOption, 0, len(memberships))
	for _, m := range memberships {
		ws, err := s.spaces.GetWorkspace(ctx, m.WorkspaceID)
		if err != nil {
			return nil, err
		}
		if !ws.IsActive {
			continue
		}
		out = append(out, WorkspaceOption{ID: ws.ID.String(), Name: ws.Name, Kind: string(ws.Kind), Slug: ws.Slug})
	}
	return out, nil
}

func (s *Service) issue(ctx context.Context, user identity.User, m identity.Membership) (LoginResult, error) {
	session, err := s.session(ctx, user, m)
	if err != nil {
		return LoginResult{}, err
	}
	token, ttl, err := s.tokens.Issue(ctxkey.Principal{
		UserID:       user.ID.String(),
		WorkspaceID:  m.WorkspaceID.String(),
		MembershipID: m.ID.String(),
		Role:         m.Role,
		Department:   m.Department,
		Scope:        session.Scope,
	}, s.clock.Now())
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		Token:     token,
		ExpiresIn: int(ttl.Seconds()),
		Session:   &session,
	}, nil
}

func (s *Service) session(ctx context.Context, user identity.User, m identity.Membership) (SessionPayload, error) {
	ws, err := s.spaces.GetWorkspace(ctx, m.WorkspaceID)
	if err != nil {
		return SessionPayload{}, err
	}
	if !ws.IsActive {
		return SessionPayload{}, errx.Forbidden("workspace_inactive", "workspace is inactive")
	}
	cat, err := s.registry.Catalog(ws.Kind)
	if err != nil {
		return SessionPayload{}, err
	}
	actor := staffscope.FromMembership(m)
	scope := staffscope.Resolve(actor, cat)
	tabs := staffscope.VisibleTabs(scope, cat)
	tabIDs := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		tabIDs = append(tabIDs, string(tab))
	}
	surface := "manager"
	if scope == staffscope.StaffPersonnel {
		surface = "staff_portal"
	}
	email := user.Email
	if strings.HasSuffix(email, "@staff.internal") {
		email = ""
	}
	return SessionPayload{
		User: UserPayload{
			ID:        user.ID.String(),
			Email:     email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		},
		Membership: MembershipPayload{
			ID:              m.ID.String(),
			Role:            m.Role,
			StaffDepartment: m.Department,
			EmployeeNumber:  m.EmployeeNumber,
		},
		Workspace: WorkspacePayload{
			ID:   ws.ID.String(),
			Name: ws.Name,
			Kind: string(ws.Kind),
			Slug: ws.Slug,
		},
		Scope:      string(scope),
		ScopeLabel: staffscope.Label(scope, m.Department),
		Tabs:       tabIDs,
		DefaultTab: string(staffscope.DefaultTab(scope, cat)),
		Surface:    surface,
	}, nil
}
