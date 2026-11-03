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

type WasteStore interface {
	ListWasteTickets(ctx context.Context, workspaceID id.ID, businessDate string) ([]cafe.WasteTicket, error)
}

// WasteSignal is live only from recorded tickets. An empty day stays unavailable.
// Count stays 0 so recorded fire never nags as a decision card.
type WasteSignal struct{ Store WasteStore }

func (WasteSignal) Kind() tenancy.SignalKind { return tenancy.SignalWaste }

func (s WasteSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	if s.Store == nil {
		return executive.Signal{Kind: tenancy.SignalWaste, Available: false, ReasonKey: "brief.sourceNotWired"}, nil
	}
	day := finance.BusinessDate(opts.Now, opts.Timezone)
	tickets, err := s.Store.ListWasteTickets(ctx, opts.WorkspaceID, day)
	if err != nil {
		return executive.Signal{}, err
	}
	sum := cafe.SummarizeWaste(day, tickets)
	if !sum.Live() {
		return executive.Signal{Kind: tenancy.SignalWaste, Available: false, ReasonKey: "dash.wasteNotWired"}, nil
	}
	return executive.Signal{
		Kind:        tenancy.SignalWaste,
		SourceKey:   string(tenancy.SignalWaste),
		Fingerprint: "tickets=" + strconv.Itoa(sum.TicketCount) + ",grams=" + strconv.FormatInt(sum.Grams, 10) + ",cost=" + strconv.FormatInt(sum.Cost, 10),
		Available:   true,
		Severity:    "medium",
		TitleKey:    "brief.wasteRecorded",
		Count:       0,
		Priority:    18,
		TargetTab:   tenancy.TabDashboard,
		Pulse: &executive.Pulse{
			WasteTickets: sum.TicketCount,
			WasteGrams:   sum.Grams,
			WasteCost:    sum.Cost,
		},
	}, nil
}
