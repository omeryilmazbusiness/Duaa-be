package brief

import (
	"context"
	"strconv"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// VoiceSource is the cafe QR inbox. Missing codes stay unavailable — never a fake card.
type VoiceSource interface {
	HasWiredSource(ctx context.Context, workspaceID id.ID) (bool, error)
	OpenRecoveries(ctx context.Context, workspaceID id.ID, now time.Time) (open, late int, err error)
}

type VoiceSignal struct{ Source VoiceSource }

func (VoiceSignal) Kind() tenancy.SignalKind { return tenancy.SignalCustomerVoice }

func (s VoiceSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	if s.Source == nil {
		return executive.Signal{Kind: tenancy.SignalCustomerVoice, Available: false, ReasonKey: "brief.sourceNotWired"}, nil
	}
	open, late, err := s.Source.OpenRecoveries(ctx, opts.WorkspaceID, opts.Now)
	if err != nil {
		return executive.Signal{}, err
	}
	wired, err := s.Source.HasWiredSource(ctx, opts.WorkspaceID)
	if err != nil {
		return executive.Signal{}, err
	}
	if !wired && open == 0 {
		return executive.Signal{Kind: tenancy.SignalCustomerVoice, Available: false, ReasonKey: "brief.sourceNotWired"}, nil
	}
	sev := "low"
	if late > 0 {
		sev = "high"
	} else if open > 0 {
		sev = "medium"
	}
	return executive.Signal{
		Kind:        tenancy.SignalCustomerVoice,
		SourceKey:   string(tenancy.SignalCustomerVoice),
		Fingerprint: "open=" + strconv.Itoa(open) + ",late=" + strconv.Itoa(late),
		Available:   true,
		Severity:    sev,
		TitleKey:    "brief.reviewInbox",
		Detail:      strconv.Itoa(open),
		Count:       open,
		Priority:    12,
		TargetTab:   tenancy.TabFeedback,
		Pulse:       &executive.Pulse{OpenRecoveries: open, LateRecoveries: late},
	}, nil
}

func severity(n int) string {
	if n > 0 {
		return "high"
	}
	return "low"
}
