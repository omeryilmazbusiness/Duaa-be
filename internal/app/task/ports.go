package task

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	CreateTask(ctx context.Context, t work.Task) error
	GetTask(ctx context.Context, id id.ID) (work.Task, error)
	ListTasksByWorkspace(ctx context.Context, workspaceID id.ID) ([]work.Task, error)
	ListTasksByAssignee(ctx context.Context, workspaceID, assignee id.ID) ([]work.Task, error)
	UpdateTask(ctx context.Context, t work.Task) error
	CreateRoutine(ctx context.Context, r work.Routine) error
	ListRoutinesByWorkspace(ctx context.Context, workspaceID id.ID) ([]work.Routine, error)
}

type MembershipReader interface {
	ListWorkspaceMemberships(ctx context.Context, workspaceID id.ID) ([]identity.Membership, error)
}
