package memory

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func closeKey(workspaceID id.ID, date string) string {
	return workspaceID.String() + ":" + date
}

func (s *Store) GetCloseByDate(_ context.Context, workspaceID id.ID, businessDate string) (finance.Close, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.closes[closeKey(workspaceID, businessDate)]
	if !ok {
		return finance.Close{}, errx.NotFound("cash_not_closed", "till is not closed for this date")
	}
	return c, nil
}

func (s *Store) CreateClose(_ context.Context, c finance.Close) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := closeKey(c.WorkspaceID, c.BusinessDate)
	if _, ok := s.closes[key]; ok {
		return errx.Conflict("cash_exists", "till already closed for this date")
	}
	s.closes[key] = c
	return nil
}

func (s *Store) UpdateClose(_ context.Context, c finance.Close) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := closeKey(c.WorkspaceID, c.BusinessDate)
	if _, ok := s.closes[key]; !ok {
		return errx.NotFound("cash_not_closed", "till is not closed for this date")
	}
	s.closes[key] = c
	return nil
}

func (s *Store) CreateApproval(_ context.Context, a finance.Approval) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals[a.ID] = a
	return nil
}

func (s *Store) GetApproval(_ context.Context, aid id.ID) (finance.Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.approvals[aid]
	if !ok {
		return finance.Approval{}, errx.NotFound("approval_not_found", "approval not found")
	}
	return a, nil
}

func (s *Store) ListApprovals(_ context.Context, workspaceID id.ID) ([]finance.Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]finance.Approval, 0)
	for _, a := range s.approvals {
		if a.WorkspaceID == workspaceID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *Store) UpdateApproval(_ context.Context, a finance.Approval) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.approvals[a.ID]; !ok {
		return errx.NotFound("approval_not_found", "approval not found")
	}
	s.approvals[a.ID] = a
	return nil
}
