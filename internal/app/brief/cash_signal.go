package brief

import (
	"context"
	"errors"
	"strconv"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type CashStore interface {
	GetCloseByDate(ctx context.Context, workspaceID id.ID, businessDate string) (finance.Close, error)
}

// CashSignal raises a morning card only when today's till is closed and does not match expected.
type CashSignal struct {
	Closes CashStore
}

func (CashSignal) Kind() tenancy.SignalKind { return tenancy.SignalCashVariance }

func (s CashSignal) Collect(ctx context.Context, opts CollectOpts) (executive.Signal, error) {
	c, err := s.Closes.GetCloseByDate(ctx, opts.WorkspaceID, finance.BusinessDate(opts.Now, opts.Timezone))
	if errors.Is(err, errx.ErrNotFound) {
		return executive.Signal{
			Kind:      tenancy.SignalCashVariance,
			Available: false,
			ReasonKey: "brief.cashNotClosed",
		}, nil
	}
	if err != nil {
		return executive.Signal{}, err
	}
	sig := executive.Signal{
		Kind:        tenancy.SignalCashVariance,
		SourceKey:   string(tenancy.SignalCashVariance),
		Fingerprint: "variance=" + strconv.FormatInt(c.Variance, 10),
		Available:   true,
		Severity:    "low",
		TitleKey:    "brief.cashVariance",
		Detail:      strconv.FormatInt(c.Variance, 10),
		Count:       0,
		Priority:    20,
		TargetTab:   tenancy.TabDashboard,
	}
	if c.HasVariance() {
		sig.Count = 1
		sig.Severity = "high"
	}
	return sig, nil
}
