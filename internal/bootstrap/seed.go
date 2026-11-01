package bootstrap

import (
	"context"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/platform/authn"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// Stable memory-seed IDs so a stored AI key survives process restarts
// (the ciphertext is bound to the workspace id).
var (
	seedOrgID  = id.MustParse("11111111-1111-4111-8111-111111111111")
	seedCafeID = id.MustParse("22222222-2222-4222-8222-222222222222")
	seedGMID   = id.MustParse("33333333-3333-4333-8333-333333333333")
	seedMemID  = id.MustParse("44444444-4444-4444-8444-444444444444")
)

type SeedStore interface {
	CreateOrganization(ctx context.Context, o tenancy.Organization) error
	CreateWorkspace(ctx context.Context, w tenancy.Workspace) error
	CreateUser(ctx context.Context, u identity.User) error
	CreateMembership(ctx context.Context, m identity.Membership) error
}

type SeedResult struct {
	OrganizationID id.ID
	CafeID         id.ID
	GMEmail        string
	GMPassword     string
}

// SeedCafe boots the first vertical only. Hotel/supply catalogs stay registered
// so those workspaces can be attached later without a schema change.
func SeedCafe(ctx context.Context, store SeedStore, now time.Time) (SeedResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	org := tenancy.Organization{ID: seedOrgID, Name: "Duaa Al-Hayani", CreatedAt: now, UpdatedAt: now}
	if err := store.CreateOrganization(ctx, org); err != nil {
		return SeedResult{}, err
	}
	cafe, err := tenancy.NewWorkspace(org.ID, tenancy.KindCafe, "Duaa Cafe", "duaa-cafe", "Asia/Baghdad", now)
	if err != nil {
		return SeedResult{}, err
	}
	cafe.ID = seedCafeID
	if err := store.CreateWorkspace(ctx, cafe); err != nil {
		return SeedResult{}, err
	}
	hash, err := authn.HashPassword("ChangeMe123!")
	if err != nil {
		return SeedResult{}, err
	}
	gm := identity.User{
		ID:           seedGMID,
		Email:        "duaa@example.com",
		PasswordHash: hash,
		FirstName:    "Duaa",
		LastName:     "Al-Hayani",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := store.CreateUser(ctx, gm); err != nil {
		return SeedResult{}, err
	}
	if err := store.CreateMembership(ctx, identity.Membership{
		ID: seedMemID, WorkspaceID: cafe.ID, UserID: gm.ID,
		Role: identity.RoleManager, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return SeedResult{}, err
	}
	return SeedResult{
		OrganizationID: org.ID,
		CafeID:         cafe.ID,
		GMEmail:        gm.Email,
		GMPassword:     "ChangeMe123!",
	}, nil
}
