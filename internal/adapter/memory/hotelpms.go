package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedHotel struct {
	Configs   []hotel.OHIPConfig  `json:"configs"`
	Snapshots []hotel.PMSSnapshot `json:"snapshots"`
	Profiles  []hotel.Profile     `json:"profiles,omitempty"`
}

func (s *Store) GetHotelProfile(_ context.Context, workspaceID id.ID) (hotel.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.hotelProfiles[workspaceID]
	if !ok {
		return hotel.Profile{}, hotel.ProfileNotFoundError()
	}
	return p, nil
}

func (s *Store) SaveHotelProfile(_ context.Context, p hotel.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hotelProfiles == nil {
		s.hotelProfiles = map[id.ID]hotel.Profile{}
	}
	s.hotelProfiles[p.WorkspaceID] = p
	return s.flushHotelLocked()
}

func pmsKey(workspaceID id.ID, date string) string {
	return workspaceID.String() + ":" + date
}

func (s *Store) AttachHotelPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hotelPersist = path
	return s.loadHotelLocked()
}

func (s *Store) GetOHIPConfig(_ context.Context, workspaceID id.ID) (hotel.OHIPConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.ohipConfigs[workspaceID]
	if !ok {
		return hotel.OHIPConfig{}, hotel.OHIPConfigNotFoundError()
	}
	return cfg, nil
}

func (s *Store) SaveOHIPConfig(_ context.Context, cfg hotel.OHIPConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ohipConfigs == nil {
		s.ohipConfigs = map[id.ID]hotel.OHIPConfig{}
	}
	s.ohipConfigs[cfg.WorkspaceID] = cfg
	return s.flushHotelLocked()
}

func (s *Store) GetPMSSnapshot(_ context.Context, workspaceID id.ID, businessDate string) (hotel.PMSSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.pmsSnapshots[pmsKey(workspaceID, businessDate)]
	if !ok {
		return hotel.PMSSnapshot{}, hotel.PMSSnapshotNotFoundError()
	}
	return snap, nil
}

func (s *Store) SavePMSSnapshot(_ context.Context, snap hotel.PMSSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pmsSnapshots == nil {
		s.pmsSnapshots = map[string]hotel.PMSSnapshot{}
	}
	s.pmsSnapshots[pmsKey(snap.WorkspaceID, snap.BusinessDate)] = snap
	return s.flushHotelLocked()
}

func (s *Store) ListPMSSnapshots(_ context.Context, workspaceID id.ID, limit int) ([]hotel.PMSSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]hotel.PMSSnapshot, 0)
	for _, snap := range s.pmsSnapshots {
		if snap.WorkspaceID == workspaceID {
			out = append(out, snap)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].BusinessDate > out[j].BusinessDate
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) flushHotelLocked() error {
	if s.hotelPersist == "" {
		return nil
	}
	payload := persistedHotel{
		Configs:   make([]hotel.OHIPConfig, 0, len(s.ohipConfigs)),
		Snapshots: make([]hotel.PMSSnapshot, 0, len(s.pmsSnapshots)),
	}
	for _, cfg := range s.ohipConfigs {
		payload.Configs = append(payload.Configs, cfg)
	}
	for _, snap := range s.pmsSnapshots {
		payload.Snapshots = append(payload.Snapshots, snap)
	}
	for _, p := range s.hotelProfiles {
		payload.Profiles = append(payload.Profiles, p)
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.hotelPersist), 0o700); err != nil {
		return err
	}
	tmp := s.hotelPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.hotelPersist)
}

func (s *Store) loadHotelLocked() error {
	if s.hotelPersist == "" {
		return nil
	}
	raw, err := os.ReadFile(s.hotelPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload persistedHotel
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	s.ohipConfigs = map[id.ID]hotel.OHIPConfig{}
	s.pmsSnapshots = map[string]hotel.PMSSnapshot{}
	s.hotelProfiles = map[id.ID]hotel.Profile{}
	for _, p := range payload.Profiles {
		if p.WorkspaceID != id.Zero() {
			s.hotelProfiles[p.WorkspaceID] = p
		}
	}
	for _, cfg := range payload.Configs {
		if cfg.WorkspaceID != id.Zero() {
			s.ohipConfigs[cfg.WorkspaceID] = cfg
		}
	}
	for _, snap := range payload.Snapshots {
		if snap.WorkspaceID != id.Zero() && snap.BusinessDate != "" {
			s.pmsSnapshots[pmsKey(snap.WorkspaceID, snap.BusinessDate)] = snap
		}
	}
	return nil
}
