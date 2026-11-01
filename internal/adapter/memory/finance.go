package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

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

type persistedApprovals struct {
	Packs []persistedApproval `json:"packs"`
}

type persistedApproval struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspaceId"`
	CreatedByUserID string    `json:"createdByUserId"`
	Title           string    `json:"title"`
	Supplier        string    `json:"supplier"`
	Amount          int64     `json:"amount"`
	Currency        string    `json:"currency"`
	Evidence        string    `json:"evidence"`
	Fingerprint     string    `json:"fingerprint"`
	Version         int       `json:"version"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (s *Store) AttachApprovalPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvalPersist = path
	return s.loadApprovalsLocked()
}

func (s *Store) CreateApproval(_ context.Context, a finance.Approval) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingFingerprintLocked(a) {
		return finance.DuplicateConflict()
	}
	if s.approvals == nil {
		s.approvals = map[id.ID]finance.Approval{}
	}
	s.approvals[a.ID] = a
	return s.flushApprovalsLocked()
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
	if a.Pending() && s.pendingFingerprintLocked(a) {
		return finance.DuplicateConflict()
	}
	s.approvals[a.ID] = a
	return s.flushApprovalsLocked()
}

func (s *Store) pendingFingerprintLocked(candidate finance.Approval) bool {
	existing := make([]finance.Approval, 0, len(s.approvals))
	for _, a := range s.approvals {
		existing = append(existing, a)
	}
	return finance.PendingDuplicate(existing, candidate)
}

func (s *Store) loadApprovalsLocked() error {
	if s.approvalPersist == "" {
		return nil
	}
	if s.approvals == nil {
		s.approvals = map[id.ID]finance.Approval{}
	}
	raw, err := os.ReadFile(s.approvalPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedApprovals
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, row := range file.Packs {
		aid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		uid, _ := id.Parse(row.CreatedByUserID)
		a := finance.Approval{
			ID: aid, WorkspaceID: wid, CreatedByUserID: uid,
			Title: row.Title, Supplier: row.Supplier, Amount: row.Amount, Currency: row.Currency,
			Evidence: row.Evidence, Fingerprint: row.Fingerprint, Version: row.Version, Status: row.Status,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
		if a.Version < 1 {
			a.Version = 1
		}
		if a.Fingerprint == "" {
			a.Fingerprint = finance.PackFingerprint(a.Title, a.Supplier, a.Amount, a.Currency)
		}
		s.approvals[aid] = a
	}
	return nil
}

func (s *Store) flushApprovalsLocked() error {
	if s.approvalPersist == "" {
		return nil
	}
	file := persistedApprovals{Packs: make([]persistedApproval, 0, len(s.approvals))}
	for _, a := range s.approvals {
		file.Packs = append(file.Packs, persistedApproval{
			ID: a.ID.String(), WorkspaceID: a.WorkspaceID.String(), CreatedByUserID: a.CreatedByUserID.String(),
			Title: a.Title, Supplier: a.Supplier, Amount: a.Amount, Currency: a.Currency,
			Evidence: a.Evidence, Fingerprint: a.Fingerprint, Version: a.Version, Status: a.Status,
			CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.approvalPersist), 0o700); err != nil {
		return err
	}
	tmp := s.approvalPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.approvalPersist)
}
