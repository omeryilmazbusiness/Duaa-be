package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedPrep struct {
	Tickets []persistedPrepTicket `json:"tickets"`
}

type persistedPrepTicket struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	OpenedBy     string    `json:"openedBy"`
	ReadyBy      string    `json:"readyBy,omitempty"`
	ServedBy     string    `json:"servedBy,omitempty"`
	BusinessDate string    `json:"businessDate"`
	Station      string    `json:"station"`
	TicketRef    string    `json:"ticketRef"`
	TableLabel   string    `json:"tableLabel,omitempty"`
	ReceivedAt   time.Time `json:"receivedAt"`
	ReadyAt      time.Time `json:"readyAt,omitempty"`
	ServedAt     time.Time `json:"servedAt,omitempty"`
}

func (s *Store) AttachPrepPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prepPersist = path
	return s.loadPrepLocked()
}

func (s *Store) CreatePrepTicket(_ context.Context, t cafe.PrepTicket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prepTickets == nil {
		s.prepTickets = map[id.ID]cafe.PrepTicket{}
	}
	for _, existing := range s.prepTickets {
		if existing.WorkspaceID == t.WorkspaceID && existing.BusinessDate == t.BusinessDate && existing.TicketRef == t.TicketRef {
			return cafe.DuplicatePrepConflict()
		}
	}
	s.prepTickets[t.ID] = t
	return s.flushPrepLocked()
}

func (s *Store) GetPrepTicket(_ context.Context, tid id.ID) (cafe.PrepTicket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.prepTickets[tid]
	if !ok {
		return cafe.PrepTicket{}, cafe.PrepNotFoundError()
	}
	return t, nil
}

func (s *Store) ListPrepTickets(_ context.Context, workspaceID id.ID, businessDate string) ([]cafe.PrepTicket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.PrepTicket, 0)
	for _, t := range s.prepTickets {
		if t.WorkspaceID == workspaceID && t.BusinessDate == businessDate {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *Store) SavePrepTicket(_ context.Context, t cafe.PrepTicket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.prepTickets[t.ID]; !ok {
		return cafe.PrepNotFoundError()
	}
	s.prepTickets[t.ID] = t
	return s.flushPrepLocked()
}

func (s *Store) loadPrepLocked() error {
	if s.prepPersist == "" {
		return nil
	}
	if s.prepTickets == nil {
		s.prepTickets = map[id.ID]cafe.PrepTicket{}
	}
	raw, err := os.ReadFile(s.prepPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedPrep
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
		opened, _ := id.Parse(row.OpenedBy)
		readyBy, _ := id.Parse(row.ReadyBy)
		servedBy, _ := id.Parse(row.ServedBy)
		s.prepTickets[tid] = cafe.PrepTicket{
			ID: tid, WorkspaceID: wid, OpenedBy: opened, ReadyBy: readyBy, ServedBy: servedBy,
			BusinessDate: row.BusinessDate, Station: cafe.PrepStation(row.Station), TicketRef: row.TicketRef,
			TableLabel: row.TableLabel, ReceivedAt: row.ReceivedAt, ReadyAt: row.ReadyAt, ServedAt: row.ServedAt,
		}
	}
	return nil
}

func (s *Store) flushPrepLocked() error {
	if s.prepPersist == "" {
		return nil
	}
	file := persistedPrep{Tickets: make([]persistedPrepTicket, 0, len(s.prepTickets))}
	for _, t := range s.prepTickets {
		file.Tickets = append(file.Tickets, persistedPrepTicket{
			ID: t.ID.String(), WorkspaceID: t.WorkspaceID.String(), OpenedBy: t.OpenedBy.String(),
			ReadyBy: idString(t.ReadyBy), ServedBy: idString(t.ServedBy), BusinessDate: t.BusinessDate,
			Station: string(t.Station), TicketRef: t.TicketRef, TableLabel: t.TableLabel,
			ReceivedAt: t.ReceivedAt, ReadyAt: t.ReadyAt, ServedAt: t.ServedAt,
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.prepPersist), 0o700); err != nil {
		return err
	}
	tmp := s.prepPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.prepPersist)
}

func idString(v id.ID) string {
	if v == id.Zero() {
		return ""
	}
	return v.String()
}
