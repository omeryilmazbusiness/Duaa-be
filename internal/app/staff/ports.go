package staff

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type UserWriter interface {
	CreateUser(ctx context.Context, u identity.User) error
	GetUser(ctx context.Context, id id.ID) (identity.User, error)
}

type MembershipRepo interface {
	CreateMembership(ctx context.Context, m identity.Membership) error
	GetMembership(ctx context.Context, id id.ID) (identity.Membership, error)
	ListWorkspaceMemberships(ctx context.Context, workspaceID id.ID) ([]identity.Membership, error)
	UpdateMembership(ctx context.Context, m identity.Membership) error
	GetActiveDepartmentManager(ctx context.Context, workspaceID id.ID, department string) (identity.Membership, error)
}

type PasswordHasher interface {
	Hash(plain string) (string, error)
}
