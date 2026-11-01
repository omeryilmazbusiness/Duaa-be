package brief

import (
	"context"
	"errors"
	"strconv"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type ChecklistStore interface {
	GetChecklistTemplate(ctx context.Context, workspaceID id.ID) (cafe.ChecklistTemplate, error)
	GetChecklistRun(ctx context.Context, workspaceID id.ID, businessDate string, kind cafe.CheckKind) (cafe.ChecklistRun, error)
}

// ChecklistSignal is a live pulse, never a nag card. An empty list is valid.
type ChecklistSignal struct{ Store ChecklistStore }

func (ChecklistSignal) Kind() tenancy.SignalKind { return tenancy.SignalChecklist }

func (s ChecklistSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	if s.Store == nil {
		return executive.Signal{Kind: tenancy.SignalChecklist, Available: false, ReasonKey: "brief.sourceNotWired"}, nil
	}
	tpl, err := s.Store.GetChecklistTemplate(ctx, opts.WorkspaceID)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return executive.Signal{}, err
	}
	day := finance.BusinessDate(opts.Now, opts.Timezone)
	openItems := tpl.ActiveItems(cafe.CheckOpen)
	closeItems := tpl.ActiveItems(cafe.CheckClose)
	openRun, err := s.Store.GetChecklistRun(ctx, opts.WorkspaceID, day, cafe.CheckOpen)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return executive.Signal{}, err
	}
	closeRun, err := s.Store.GetChecklistRun(ctx, opts.WorkspaceID, day, cafe.CheckClose)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return executive.Signal{}, err
	}
	return executive.Signal{
		Kind:        tenancy.SignalChecklist,
		SourceKey:   string(tenancy.SignalChecklist),
		Fingerprint: "open=" + strconv.Itoa(openRun.DoneCount(openItems)) + "/" + strconv.Itoa(len(openItems)) + ",close=" + strconv.Itoa(closeRun.DoneCount(closeItems)) + "/" + strconv.Itoa(len(closeItems)),
		Available:   true,
		TitleKey:    "brief.checklist",
		Count:       0,
		Priority:    42,
		TargetTab:   tenancy.TabFeedback,
		Pulse: &executive.Pulse{
			OpenChecks:  len(openItems),
			OpenDone:    openRun.DoneCount(openItems),
			CloseChecks: len(closeItems),
			CloseDone:   closeRun.DoneCount(closeItems),
		},
	}, nil
}
