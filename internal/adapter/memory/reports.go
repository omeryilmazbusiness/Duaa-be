package memory

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) CreateReport(_ context.Context, r opsreport.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports[r.ID] = r
	return nil
}

func (s *Store) GetReport(_ context.Context, rid id.ID) (opsreport.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.reports[rid]
	if !ok {
		return opsreport.Report{}, errx.NotFound("report_not_found", "report not found")
	}
	return r, nil
}

func (s *Store) ListReports(_ context.Context, workspaceID id.ID) ([]opsreport.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]opsreport.Report, 0)
	for _, r := range s.reports {
		if r.WorkspaceID == workspaceID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) UpdateReport(_ context.Context, r opsreport.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.reports[r.ID]; !ok {
		return errx.NotFound("report_not_found", "report not found")
	}
	s.reports[r.ID] = r
	return nil
}
