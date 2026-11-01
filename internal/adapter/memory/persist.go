package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedAI struct {
	Settings []persistedSettings `json:"settings"`
}

type persistedSettings struct {
	WorkspaceID     string    `json:"workspaceId"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	EncryptedKey    string    `json:"encryptedKey"`
	KeyHint         string    `json:"keyHint"`
	DailyLimit      int       `json:"dailyLimit"`
	Enabled         bool      `json:"enabled"`
	UpdatedByUserID string    `json:"updatedByUserId"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// AttachAIPersist loads sealed assistant settings from path and writes them
// back on every save. The file holds ciphertext only.
func (s *Store) AttachAIPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiPersist = path
	return s.loadAILocked()
}

func (s *Store) loadAILocked() error {
	if s.aiPersist == "" {
		return nil
	}
	raw, err := os.ReadFile(s.aiPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedAI
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, row := range file.Settings {
		wid, err := id.Parse(row.WorkspaceID)
		if err != nil || row.EncryptedKey == "" {
			continue
		}
		uid, _ := id.Parse(row.UpdatedByUserID)
		s.aiSettings[wid] = assistant.Settings{
			WorkspaceID: wid, Provider: assistant.Provider(row.Provider), Model: row.Model,
			EncryptedKey: row.EncryptedKey, KeyHint: row.KeyHint, DailyLimit: row.DailyLimit,
			Enabled: row.Enabled, UpdatedByUserID: uid, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
	}
	return nil
}

func (s *Store) flushAILocked() error {
	if s.aiPersist == "" {
		return nil
	}
	file := persistedAI{Settings: make([]persistedSettings, 0, len(s.aiSettings))}
	for _, v := range s.aiSettings {
		file.Settings = append(file.Settings, persistedSettings{
			WorkspaceID: v.WorkspaceID.String(), Provider: string(v.Provider), Model: v.Model,
			EncryptedKey: v.EncryptedKey, KeyHint: v.KeyHint, DailyLimit: v.DailyLimit, Enabled: v.Enabled,
			UpdatedByUserID: v.UpdatedByUserID.String(), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
		})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.aiPersist), 0o700); err != nil {
		return err
	}
	tmp := s.aiPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.aiPersist)
}
