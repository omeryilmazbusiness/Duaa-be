package cash

import (
	"context"
	"errors"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	store Repository
	clock clock.Clock
}

func New(store Repository, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{store: store, clock: clk}
}

type View struct {
	ID             string `json:"id"`
	BusinessDate   string `json:"businessDate"`
	CountedAmount  int64  `json:"countedAmount"`
	ExpectedAmount *int64 `json:"expectedAmount,omitempty"`
	Variance       int64  `json:"variance"`
	Note           string `json:"note,omitempty"`
	ClosedAt       string `json:"closedAt"`
}

func toView(c finance.Close) View {
	return View{
		ID:             c.ID.String(),
		BusinessDate:   c.BusinessDate,
		CountedAmount:  c.CountedAmount,
		ExpectedAmount: c.ExpectedAmount,
		Variance:       c.Variance,
		Note:           c.Note,
		ClosedAt:       c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Service) Close(ctx context.Context, ac actor.Context, counted int64, expected *int64, note string) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.CloseCash) {
		return View{}, errx.Forbidden("forbidden", "only the general manager closes the till")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	now := s.clock.Now()
	date := finance.BusinessDate(now, ac.Workspace.Timezone)
	existing, err := s.store.GetCloseByDate(ctx, ac.Workspace.ID, date)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	if errors.Is(err, errx.ErrNotFound) {
		c, err := finance.NewClose(ac.Workspace.ID, uid, date, counted, expected, note, now)
		if err != nil {
			return View{}, err
		}
		if err := s.store.CreateClose(ctx, c); err != nil {
			return View{}, err
		}
		return toView(c), nil
	}
	if err := existing.Recount(counted, expected, note, now); err != nil {
		return View{}, err
	}
	if err := s.store.UpdateClose(ctx, existing); err != nil {
		return View{}, err
	}
	return toView(existing), nil
}

func (s *Service) Today(ctx context.Context, ac actor.Context) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.ViewFinance) {
		return View{}, errx.Forbidden("forbidden", "finance is a general manager surface")
	}
	date := finance.BusinessDate(s.clock.Now(), ac.Workspace.Timezone)
	c, err := s.store.GetCloseByDate(ctx, ac.Workspace.ID, date)
	if err != nil {
		return View{}, err
	}
	return toView(c), nil
}
