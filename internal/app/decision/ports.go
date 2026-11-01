package decision

import (
	"context"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	CreateDecision(ctx context.Context, d executive.Decision) error
	GetDecision(ctx context.Context, id id.ID) (executive.Decision, error)
	ListDecisions(ctx context.Context, workspaceID id.ID) ([]executive.Decision, error)
	UpdateDecision(ctx context.Context, d executive.Decision) error
}

type TaskStatusReader interface {
	GetTask(ctx context.Context, id id.ID) (work.Task, error)
}

// AssignmentPort is implemented in bootstrap so this module does not import app/task (DIP).
type AssignmentPort interface {
	Assign(ctx context.Context, ac actor.Context, title, description, assigneeUserID string, start, end time.Time) (id.ID, error)
}
