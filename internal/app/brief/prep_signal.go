package brief

import (
	"context"
	"strconv"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type PrepStore interface {
	ListPrepTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.PrepTicket, error)
}

// PrepSignal is live only from recorded tickets. An empty day stays unavailable.
type PrepSignal struct{ Store PrepStore }

func (PrepSignal) Kind() tenancy.SignalKind { return tenancy.SignalPrepTime }

func (s PrepSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	if s.Store == nil {
		return executive.Signal{Kind: tenancy.SignalPrepTime, Available: false, ReasonKey: "brief.sourceNotWired"}, nil
	}
	day := finance.BusinessDate(opts.Now, opts.Timezone)
	tickets, err := s.Store.ListPrepTickets(ctx, opts.WorkspaceID, day)
	if err != nil {
		return executive.Signal{}, err
	}
	sum := cafe.SummarizePrep(day, tickets, opts.Now.UTC())
	if !sum.Live() {
		return executive.Signal{Kind: tenancy.SignalPrepTime, Available: false, ReasonKey: "dash.prepNotWired"}, nil
	}
	severity := "medium"
	if sum.WaitingCount > 0 {
		severity = "high"
	}
	return executive.Signal{
		Kind:        tenancy.SignalPrepTime,
		SourceKey:   string(tenancy.SignalPrepTime),
		Fingerprint: "open=" + strconv.Itoa(sum.OpenCount()) + ",wait=" + strconv.Itoa(sum.WaitingCount) + ",longest=" + strconv.Itoa(sum.LongestOpenMinutes),
		Available:   true,
		Severity:    severity,
		TitleKey:    "brief.prepWaiting",
		Count:       sum.OpenCount(),
		Priority:    16,
		TargetTab:   tenancy.TabToday,
		Pulse: &executive.Pulse{
			WaitingPrep: sum.WaitingCount,
			ReadyPrep:   sum.ReadyCount,
			ServedPrep:  sum.ServedCount,
		},
	}, nil
}
