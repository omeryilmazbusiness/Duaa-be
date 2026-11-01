package brief

import (
	"context"
	"strconv"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type ReportStore interface {
	ListReports(ctx context.Context, workspaceID id.ID) ([]opsreport.Report, error)
}

// BreakdownSignal is open arıza only. Requests never become a fake breakage card.
type BreakdownSignal struct{ Store ReportStore }

func (BreakdownSignal) Kind() tenancy.SignalKind { return tenancy.SignalBreakdown }

func (s BreakdownSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	if s.Store == nil {
		return executive.Signal{Kind: tenancy.SignalBreakdown, Available: false, ReasonKey: "brief.sourceNotWired"}, nil
	}
	list, err := s.Store.ListReports(ctx, opts.WorkspaceID)
	if err != nil {
		return executive.Signal{}, err
	}
	n := 0
	for _, r := range list {
		if r.WorkspaceID == opts.WorkspaceID && r.Kind == opsreport.KindBreakage && r.Open() {
			n++
		}
	}
	return executive.Signal{
		Kind:        tenancy.SignalBreakdown,
		SourceKey:   string(tenancy.SignalBreakdown),
		Fingerprint: "open=" + strconv.Itoa(n),
		Available:   true,
		Severity:    severity(n),
		TitleKey:    "brief.breakdowns",
		Count:       n,
		Priority:    14,
		TargetTab:   tenancy.TabFeedback,
		Pulse:       &executive.Pulse{OpenBreakdowns: n},
	}, nil
}
