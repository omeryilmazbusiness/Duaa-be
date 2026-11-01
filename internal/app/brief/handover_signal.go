package brief

import (
	"context"
	"errors"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type HandoverStore interface {
	GetOpenHandover(ctx context.Context, workspaceID id.ID) (cafe.Handover, error)
}

// HandoverSignal raises a card only when an open file exists. Empty pulse is not a file.
type HandoverSignal struct{ Store HandoverStore }

func (HandoverSignal) Kind() tenancy.SignalKind { return tenancy.SignalHandover }

func (s HandoverSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	if s.Store == nil {
		return executive.Signal{Kind: tenancy.SignalHandover, Available: false, ReasonKey: "brief.sourceNotWired"}, nil
	}
	rec, err := s.Store.GetOpenHandover(ctx, opts.WorkspaceID)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return executive.Signal{}, err
	}
	open := err == nil && rec.Open()
	count := 0
	if open {
		count = 1
	}
	return executive.Signal{
		Kind:        tenancy.SignalHandover,
		SourceKey:   string(tenancy.SignalHandover),
		Fingerprint: "open=" + boolBit(open),
		Available:   true,
		Severity:    severity(count),
		TitleKey:    "brief.handover",
		Count:       count,
		Priority:    5,
		TargetTab:   tenancy.TabFeedback,
		Pulse:       &executive.Pulse{HandoverOpen: open},
	}, nil
}

func boolBit(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
