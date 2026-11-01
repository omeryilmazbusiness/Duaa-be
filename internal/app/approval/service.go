package approval

import (
	"context"
	"sort"
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
	ID        string `json:"id"`
	Title     string `json:"title"`
	Supplier  string `json:"supplier"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Evidence  string `json:"evidence,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func toView(a finance.Approval) View {
	return View{
		ID:        a.ID.String(),
		Title:     a.Title,
		Supplier:  a.Supplier,
		Amount:    a.Amount,
		Currency:  a.Currency,
		Evidence:  a.Evidence,
		Status:    a.Status,
		CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Service) Submit(ctx context.Context, ac actor.Context, title, supplier, evidence string, amount int64) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.SubmitApproval) {
		return View{}, errx.Forbidden("forbidden", "managers submit spend packs")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Unauthenticated("invalid token")
	}
	a, err := finance.NewApproval(ac.Workspace.ID, uid, title, supplier, finance.CurrencyIQD, evidence, amount, s.clock.Now())
	if err != nil {
		return View{}, err
	}
	if err := s.store.CreateApproval(ctx, a); err != nil {
		return View{}, err
	}
	return toView(a), nil
}

func (s *Service) List(ctx context.Context, ac actor.Context) ([]View, error) {
	if !rbac.Allows(ac.Scope, rbac.ViewFinance) && !rbac.Allows(ac.Scope, rbac.SubmitApproval) {
		return nil, errx.Forbidden("forbidden", "cannot list spend packs")
	}
	all, err := s.store.ListApprovals(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(all))
	for _, a := range all {
		if a.WorkspaceID != ac.Workspace.ID {
			continue
		}
		out = append(out, toView(a))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func (s *Service) Act(ctx context.Context, ac actor.Context, approvalID, action string, amount *int64) (View, error) {
	if !rbac.Allows(ac.Scope, rbac.ActApproval) {
		return View{}, errx.Forbidden("forbidden", "only the general manager acts on spend packs")
	}
	aid, err := id.Parse(approvalID)
	if err != nil {
		return View{}, errx.Invalid("invalid_id", "invalid approval id")
	}
	a, err := s.store.GetApproval(ctx, aid)
	if err != nil {
		return View{}, err
	}
	if a.WorkspaceID != ac.Workspace.ID {
		return View{}, errx.NotFound("not_found", "approval not found")
	}
	now := s.clock.Now()
	if amount != nil {
		if err := a.ReviseAmount(*amount, now); err != nil {
			return View{}, err
		}
	}
	if action != "" {
		if err := a.Act(action, now); err != nil {
			return View{}, err
		}
	}
	if err := s.store.UpdateApproval(ctx, a); err != nil {
		return View{}, err
	}
	return toView(a), nil
}
