package auth

import (
	"context"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type UserReader interface {
	GetUser(ctx context.Context, id id.ID) (identity.User, error)
	GetUserByEmail(ctx context.Context, email string) (identity.User, error)
}

type MembershipReader interface {
	GetMembership(ctx context.Context, id id.ID) (identity.Membership, error)
	ListMembershipsByUser(ctx context.Context, userID id.ID) ([]identity.Membership, error)
	GetMembershipByEmployee(ctx context.Context, workspaceID id.ID, employeeNumber string) (identity.Membership, error)
}

type WorkspaceReader interface {
	GetWorkspace(ctx context.Context, id id.ID) (tenancy.Workspace, error)
	GetWorkspaceBySlug(ctx context.Context, slug string) (tenancy.Workspace, error)
}

type Tokens interface {
	Issue(p ctxkey.Principal, now time.Time) (string, time.Duration, error)
}

type PasswordChecker interface {
	Check(hash, plain string) bool
}
