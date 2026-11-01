package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedPOS struct {
	Tickets []persistedTicket   `json:"tickets"`
	Closes  []persistedPOSClose `json:"closes"`
}

type persistedTicket struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	RecordedBy   string    `json:"recordedBy"`
	ExternalRef  string    `json:"externalRef"`
	BusinessDate string    `json:"businessDate"`
	Kind         string    `json:"kind"`
	Lines        int       `json:"lines"`
	Gross        int64     `json:"gross"`
	Discount     int64     `json:"discount"`
	Net          int64     `json:"net"`
	Tender       string    `json:"tender"`
	RecordedAt   time.Time `json:"recordedAt"`
}

type persistedPOSClose struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	ClosedBy     string    `json:"closedBy"`
	BusinessDate string    `json:"businessDate"`
	TicketCount  int       `json:"ticketCount"`
	SaleCount    int       `json:"saleCount"`
	ReturnCount  int       `json:"returnCount"`
	Gross        int64     `json:"gross"`
	Discount     int64     `json:"discount"`
	Returns      int64     `json:"returns"`
	Net          int64     `json:"net"`
	Basket       int64     `json:"basket"`
	ClosedAt     time.Time `json:"closedAt"`
}

func posCloseKey(workspaceID id.ID, day string) string {
	return workspaceID.String() + ":" + day
}

func (s *Store) AttachPOSPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.posPersist = path
	return s.loadPOSLocked()
}

func (s *Store) CreatePOSTicket(_ context.Context, t cafe.Ticket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.posTickets == nil {
		s.posTickets = map[id.ID]cafe.Ticket{}
	}
	for _, existing := range s.posTickets {
		if existing.WorkspaceID == t.WorkspaceID && existing.ExternalRef == t.ExternalRef {
			return cafe.DuplicateTicketConflict()
		}
	}
	s.posTickets[t.ID] = t
	return s.flushPOSLocked()
}

func (s *Store) ListPOSTickets(_ context.Context, workspaceID id.ID, businessDate string) ([]cafe.Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.Ticket, 0)
	for _, t := range s.posTickets {
		if t.WorkspaceID == workspaceID && t.BusinessDate == businessDate {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *Store) GetPOSClose(_ context.Context, workspaceID id.ID, businessDate string) (cafe.POSClose, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.posCloses[posCloseKey(workspaceID, businessDate)]
	if !ok {
		return cafe.POSClose{}, errx.NotFound("pos_not_closed", "POS day is not closed")
	}
	return c, nil
}

func (s *Store) CreatePOSClose(_ context.Context, c cafe.POSClose) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.posCloses == nil {
		s.posCloses = map[string]cafe.POSClose{}
	}
	key := posCloseKey(c.WorkspaceID, c.BusinessDate)
	if _, ok := s.posCloses[key]; ok {
		return cafe.POSDayClosedConflict()
	}
	s.posCloses[key] = c
	return s.flushPOSLocked()
}

func (s *Store) loadPOSLocked() error {
	if s.posPersist == "" {
		return nil
	}
	if s.posTickets == nil {
		s.posTickets = map[id.ID]cafe.Ticket{}
	}
	if s.posCloses == nil {
		s.posCloses = map[string]cafe.POSClose{}
	}
	raw, err := os.ReadFile(s.posPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedPOS
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, row := range file.Tickets {
		tid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		uid, _ := id.Parse(row.RecordedBy)
		s.posTickets[tid] = cafe.Ticket{
			ID: tid, WorkspaceID: wid, RecordedBy: uid, ExternalRef: row.ExternalRef, BusinessDate: row.BusinessDate,
			Kind: cafe.TicketKind(row.Kind), Lines: row.Lines, Gross: row.Gross, Discount: row.Discount, Net: row.Net,
			Tender: cafe.TicketTender(row.Tender), RecordedAt: row.RecordedAt,
		}
	}
	for _, row := range file.Closes {
		cid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		uid, _ := id.Parse(row.ClosedBy)
		c := cafe.POSClose{
			ID: cid, WorkspaceID: wid, ClosedBy: uid, BusinessDate: row.BusinessDate,
			TicketCount: row.TicketCount, SaleCount: row.SaleCount, ReturnCount: row.ReturnCount,
			Gross: row.Gross, Discount: row.Discount, Returns: row.Returns, Net: row.Net, Basket: row.Basket,
			ClosedAt: row.ClosedAt,
		}
		s.posCloses[posCloseKey(wid, row.BusinessDate)] = c
	}
	return nil
}

func (s *Store) flushPOSLocked() error {
	if s.posPersist == "" {
		return nil
	}
	file := persistedPOS{Tickets: make([]persistedTicket, 0, len(s.posTickets)), Closes: make([]persistedPOSClose, 0, len(s.posCloses))}
	for _, t := range s.posTickets {
		file.Tickets = append(file.Tickets, persistedTicket{
			ID: t.ID.String(), WorkspaceID: t.WorkspaceID.String(), RecordedBy: t.RecordedBy.String(),
			ExternalRef: t.ExternalRef, BusinessDate: t.BusinessDate, Kind: string(t.Kind), Lines: t.Lines,
			Gross: t.Gross, Discount: t.Discount, Net: t.Net, Tender: string(t.Tender), RecordedAt: t.RecordedAt,
		})
	}
	for _, c := range s.posCloses {
		file.Closes = append(file.Closes, persistedPOSClose{
			ID: c.ID.String(), WorkspaceID: c.WorkspaceID.String(), ClosedBy: c.ClosedBy.String(),
			BusinessDate: c.BusinessDate, TicketCount: c.TicketCount, SaleCount: c.SaleCount, ReturnCount: c.ReturnCount,
			Gross: c.Gross, Discount: c.Discount, Returns: c.Returns, Net: c.Net, Basket: c.Basket, ClosedAt: c.ClosedAt,
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.posPersist), 0o700); err != nil {
		return err
	}
	tmp := s.posPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.posPersist)
}
