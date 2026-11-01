package brief

import (
	"context"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type CollectOpts struct {
	WorkspaceID id.ID
	Department  string
	Now         time.Time
	Timezone    string
}

// Port is one live (or honestly missing) morning-OS input. New verticals register a port.
type Port interface {
	Kind() tenancy.SignalKind
	Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error)
}

type TaskStore interface {
	ListTasksByWorkspace(ctx context.Context, workspaceID id.ID) ([]work.Task, error)
}

type MembershipReader interface {
	ListWorkspaceMemberships(ctx context.Context, workspaceID id.ID) ([]identity.Membership, error)
}

type DecisionReader interface {
	ListDecisions(ctx context.Context, workspaceID id.ID) ([]executive.Decision, error)
}
