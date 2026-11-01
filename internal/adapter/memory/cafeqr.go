package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedQR struct {
	Codes      []persistedCode      `json:"codes"`
	Voices     []persistedVoice     `json:"voices"`
	Recoveries []persistedRecovery  `json:"recoveries,omitempty"`
}

type persistedCode struct {
	WorkspaceID string    `json:"workspaceId"`
	SiteID      string    `json:"siteId"`
	Token       string    `json:"token"`
	IssuedAt    time.Time `json:"issuedAt"`
	RotatedAt   time.Time `json:"rotatedAt"`
}

type persistedVoice struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	SiteID       string    `json:"siteId"`
	Token        string    `json:"token"`
	Kind         string    `json:"kind"`
	Rating       int       `json:"rating"`
	Comment      string    `json:"comment"`
	ContactName  string    `json:"contactName"`
	ContactPhone string    `json:"contactPhone"`
	CreatedAt    time.Time `json:"createdAt"`
}

type persistedRecovery struct {
	ID                   string    `json:"id"`
	WorkspaceID          string    `json:"workspaceId"`
	VoiceID              string    `json:"voiceId"`
	Status               string    `json:"status"`
	AssigneeUserID       string    `json:"assigneeUserId,omitempty"`
	LinkedTaskID         string    `json:"linkedTaskId,omitempty"`
	CompensationAmount   int64     `json:"compensationAmount,omitempty"`
	CompensationCurrency string    `json:"compensationCurrency,omitempty"`
	CompensatedAt        time.Time `json:"compensatedAt,omitempty"`
	ClosedAt             time.Time `json:"closedAt,omitempty"`
	Note                 string    `json:"note,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func (s *Store) AttachQRPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.qrPersist = path
	return s.loadQRLocked()
}

func (s *Store) GetQRCodeBySite(_ context.Context, workspaceID, siteID id.ID) (cafe.Code, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.qrCodes[siteID]
	if !ok || c.WorkspaceID != workspaceID {
		return cafe.Code{}, errx.NotFound("qr_code_not_found", "QR code not found")
	}
	return c, nil
}

func (s *Store) GetQRCodeByToken(_ context.Context, token string) (cafe.Code, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid, ok := s.qrByToken[token]
	if !ok {
		return cafe.Code{}, errx.NotFound("qr_code_not_found", "QR code not found")
	}
	return s.qrCodes[sid], nil
}

func (s *Store) SaveQRCode(_ context.Context, code cafe.Code) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.qrCodes == nil {
		s.qrCodes = map[id.ID]cafe.Code{}
	}
	if s.qrByToken == nil {
		s.qrByToken = map[string]id.ID{}
	}
	if prev, ok := s.qrCodes[code.SiteID]; ok && prev.Token != "" && prev.Token != code.Token {
		delete(s.qrByToken, prev.Token)
	}
	s.qrCodes[code.SiteID] = code
	s.qrByToken[code.Token] = code.SiteID
	return s.flushQRLocked()
}

func (s *Store) ListQRCodes(_ context.Context, workspaceID id.ID) ([]cafe.Code, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.Code, 0)
	for _, c := range s.qrCodes {
		if c.WorkspaceID == workspaceID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *Store) CreateVoice(_ context.Context, v cafe.Voice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.voices == nil {
		s.voices = map[id.ID]cafe.Voice{}
	}
	s.voices[v.ID] = v
	return s.flushQRLocked()
}

func (s *Store) GetVoice(_ context.Context, voiceID id.ID) (cafe.Voice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.voices[voiceID]
	if !ok {
		return cafe.Voice{}, errx.NotFound("voice_not_found", "guest voice not found")
	}
	return v, nil
}

func (s *Store) GetRecoveryByVoice(_ context.Context, voiceID id.ID) (cafe.Recovery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.recoveries[voiceID]
	if !ok {
		return cafe.Recovery{}, errx.NotFound("recovery_not_found", "recovery not found")
	}
	return rec, nil
}

func (s *Store) ListRecoveries(_ context.Context, workspaceID id.ID) ([]cafe.Recovery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.Recovery, 0)
	for _, rec := range s.recoveries {
		if rec.WorkspaceID == workspaceID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (s *Store) SaveRecovery(_ context.Context, rec cafe.Recovery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recoveries == nil {
		s.recoveries = map[id.ID]cafe.Recovery{}
	}
	s.recoveries[rec.VoiceID] = rec
	return s.flushQRLocked()
}

func (s *Store) ListVoices(_ context.Context, workspaceID id.ID) ([]cafe.Voice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cafe.Voice, 0)
	for _, v := range s.voices {
		if v.WorkspaceID == workspaceID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) CountUnhappyVoicesSince(_ context.Context, workspaceID id.ID, since time.Time) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, v := range s.voices {
		if v.WorkspaceID == workspaceID && !v.CreatedAt.Before(since) && v.Unhappy() {
			n++
		}
	}
	return n, nil
}

func (s *Store) loadQRLocked() error {
	if s.qrPersist == "" {
		return nil
	}
	raw, err := os.ReadFile(s.qrPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedQR
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	if s.qrCodes == nil {
		s.qrCodes = map[id.ID]cafe.Code{}
	}
	if s.qrByToken == nil {
		s.qrByToken = map[string]id.ID{}
	}
	if s.voices == nil {
		s.voices = map[id.ID]cafe.Voice{}
	}
	if s.recoveries == nil {
		s.recoveries = map[id.ID]cafe.Recovery{}
	}
	for _, row := range file.Codes {
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil {
			continue
		}
		sid, err := id.Parse(row.SiteID)
		if err != nil {
			continue
		}
		c := cafe.Code{WorkspaceID: wid, SiteID: sid, Token: row.Token, IssuedAt: row.IssuedAt, RotatedAt: row.RotatedAt}
		s.qrCodes[sid] = c
		s.qrByToken[row.Token] = sid
	}
	for _, row := range file.Voices {
		vid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, _ := id.Parse(row.WorkspaceID)
		sid, _ := id.Parse(row.SiteID)
		kind, _ := cafe.ParseVoiceKind(row.Kind)
		s.voices[vid] = cafe.Voice{
			ID: vid, WorkspaceID: wid, SiteID: sid, Token: row.Token, Kind: kind, Rating: row.Rating,
			Comment: row.Comment, ContactName: row.ContactName, ContactPhone: row.ContactPhone, CreatedAt: row.CreatedAt,
		}
	}
	for _, row := range file.Recoveries {
		rid, err := id.Parse(row.ID)
		if err != nil {
			continue
		}
		wid, _ := id.Parse(row.WorkspaceID)
		vid, _ := id.Parse(row.VoiceID)
		assignee, _ := id.Parse(row.AssigneeUserID)
		taskID, _ := id.Parse(row.LinkedTaskID)
		s.recoveries[vid] = cafe.Recovery{
			ID: rid, WorkspaceID: wid, VoiceID: vid, Status: row.Status,
			AssigneeUserID: assignee, LinkedTaskID: taskID,
			CompensationAmount: row.CompensationAmount, CompensationCurrency: row.CompensationCurrency,
			CompensatedAt: row.CompensatedAt, ClosedAt: row.ClosedAt, Note: row.Note,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
	}
	return nil
}

func (s *Store) flushQRLocked() error {
	if s.qrPersist == "" {
		return nil
	}
	file := persistedQR{
		Codes:      make([]persistedCode, 0, len(s.qrCodes)),
		Voices:     make([]persistedVoice, 0, len(s.voices)),
		Recoveries: make([]persistedRecovery, 0, len(s.recoveries)),
	}
	for _, c := range s.qrCodes {
		file.Codes = append(file.Codes, persistedCode{
			WorkspaceID: c.WorkspaceID.String(), SiteID: c.SiteID.String(), Token: c.Token,
			IssuedAt: c.IssuedAt, RotatedAt: c.RotatedAt,
		})
	}
	for _, v := range s.voices {
		file.Voices = append(file.Voices, persistedVoice{
			ID: v.ID.String(), WorkspaceID: v.WorkspaceID.String(), SiteID: v.SiteID.String(), Token: v.Token,
			Kind: string(v.Kind), Rating: v.Rating, Comment: v.Comment, ContactName: v.ContactName, ContactPhone: v.ContactPhone, CreatedAt: v.CreatedAt,
		})
	}
	for _, rec := range s.recoveries {
		file.Recoveries = append(file.Recoveries, persistedRecovery{
			ID: rec.ID.String(), WorkspaceID: rec.WorkspaceID.String(), VoiceID: rec.VoiceID.String(), Status: rec.Status,
			AssigneeUserID: rec.AssigneeUserID.String(), LinkedTaskID: rec.LinkedTaskID.String(),
			CompensationAmount: rec.CompensationAmount, CompensationCurrency: rec.CompensationCurrency,
			CompensatedAt: rec.CompensatedAt, ClosedAt: rec.ClosedAt, Note: rec.Note,
			CreatedAt: rec.CreatedAt, UpdatedAt: rec.UpdatedAt,
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.qrPersist), 0o700); err != nil {
		return err
	}
	tmp := s.qrPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.qrPersist)
}
