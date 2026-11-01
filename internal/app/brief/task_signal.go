package brief

import (
	"context"
	"strconv"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// TaskSignal is the only live cafe source in Task 1. Reviews/cash are not faked here.
type TaskSignal struct {
	Tasks   TaskStore
	Members MembershipReader
}

func (TaskSignal) Kind() tenancy.SignalKind { return tenancy.SignalLateStaff }

func (s TaskSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	tasks, err := s.Tasks.ListTasksByWorkspace(ctx, opts.WorkspaceID)
	if err != nil {
		return executive.Signal{}, err
	}
	deptUsers := map[id.ID]struct{}{}
	if opts.Department != "" {
		members, err := s.Members.ListWorkspaceMemberships(ctx, opts.WorkspaceID)
		if err != nil {
			return executive.Signal{}, err
		}
		for _, m := range members {
			if m.WorkspaceID == opts.WorkspaceID && m.Department == opts.Department {
				deptUsers[m.UserID] = struct{}{}
			}
		}
	}
	pulse := executive.Pulse{}
	for _, task := range tasks {
		if task.WorkspaceID != opts.WorkspaceID {
			continue
		}
		if opts.Department != "" {
			if _, ok := deptUsers[task.AssigneeUserID]; !ok {
				continue
			}
		}
		if task.Status == work.StatusCompleted {
			if task.CompletedAt != nil && sameDay(*task.CompletedAt, opts.Now) {
				pulse.CompletedToday++
			}
			continue
		}
		if task.Status == work.StatusCancelled {
			continue
		}
		pulse.OpenTasks++
		if task.Overdue(opts.Now) {
			pulse.OverdueTasks++
		}
	}
	sig := executive.Signal{
		Kind:        tenancy.SignalLateStaff,
		SourceKey:   string(tenancy.SignalLateStaff),
		Fingerprint: "overdue=" + strconv.Itoa(pulse.OverdueTasks),
		Available:   true,
		Severity:    "low",
		TitleKey:    "brief.overdueTasks",
		Detail:      strconv.Itoa(pulse.OverdueTasks),
		Count:       pulse.OverdueTasks,
		Priority:    10,
		TargetTab:   tenancy.TabOps,
		Pulse:       &pulse,
	}
	if pulse.OverdueTasks > 0 {
		sig.Severity = "high"
	}
	return sig, nil
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}
