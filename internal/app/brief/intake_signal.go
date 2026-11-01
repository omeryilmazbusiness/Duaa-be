package brief

import (
	"context"
	"errors"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type IntakeStore interface {
	GetIntake(ctx context.Context, workspaceID id.ID, businessDate string) (cafe.Intake, error)
}

// IntakeSignal is a live pulse, never a nag card. An empty day is valid.
type IntakeSignal struct{ Store IntakeStore }

func (IntakeSignal) Kind() tenancy.SignalKind { return tenancy.SignalDailyClose }

func (s IntakeSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	if s.Store == nil {
		return executive.Signal{Kind: tenancy.SignalDailyClose, Available: false, ReasonKey: "brief.sourceNotWired"}, nil
	}
	day := finance.BusinessDate(opts.Now, opts.Timezone)
	_, err := s.Store.GetIntake(ctx, opts.WorkspaceID, day)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return executive.Signal{}, err
	}
	written := err == nil
	return executive.Signal{
		Kind:        tenancy.SignalDailyClose,
		SourceKey:   string(tenancy.SignalDailyClose),
		Fingerprint: "written=" + boolBit(written),
		Available:   true,
		TitleKey:    "brief.dailyClose",
		Count:       0,
		Priority:    40,
		TargetTab:   tenancy.TabFeedback,
		Pulse:       &executive.Pulse{IntakeWritten: written},
	}, nil
}
