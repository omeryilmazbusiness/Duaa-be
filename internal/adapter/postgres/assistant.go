package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) GetAssistantSettings(ctx context.Context, workspaceID id.ID) (assistant.Settings, error) {
	var v assistant.Settings
	var provider string
	err := s.pool.QueryRow(ctx, `SELECT workspace_id, provider, model, encrypted_key, key_hint, daily_limit, enabled,
		updated_by_user_id, created_at, updated_at FROM ai_settings WHERE workspace_id=$1`, workspaceID).Scan(
		&v.WorkspaceID, &provider, &v.Model, &v.EncryptedKey, &v.KeyHint, &v.DailyLimit, &v.Enabled,
		&v.UpdatedByUserID, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return assistant.Settings{}, mapErr(err)
	}
	v.Provider = assistant.Provider(provider)
	return v, nil
}

func (s *Store) SaveAssistantSettings(ctx context.Context, v assistant.Settings) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO ai_settings (workspace_id, provider, model, encrypted_key, key_hint, daily_limit,
		enabled, updated_by_user_id, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (workspace_id) DO UPDATE SET provider=EXCLUDED.provider, model=EXCLUDED.model,
		encrypted_key=EXCLUDED.encrypted_key, key_hint=EXCLUDED.key_hint, daily_limit=EXCLUDED.daily_limit,
		enabled=EXCLUDED.enabled, updated_by_user_id=EXCLUDED.updated_by_user_id, updated_at=EXCLUDED.updated_at`,
		v.WorkspaceID, string(v.Provider), v.Model, v.EncryptedKey, v.KeyHint, v.DailyLimit, v.Enabled,
		v.UpdatedByUserID, v.CreatedAt, v.UpdatedAt)
	return err
}

func (s *Store) DeleteAssistantSettings(ctx context.Context, workspaceID id.ID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM ai_settings WHERE workspace_id=$1`, workspaceID)
	return err
}

// ReserveAIUsage increments the day counter only while it is below limit, in a
// single statement, so concurrent requests can never overshoot.
func (s *Store) ReserveAIUsage(ctx context.Context, workspaceID id.ID, date string, limit int) (int, bool, error) {
	var used int
	err := s.pool.QueryRow(ctx, `INSERT INTO ai_usage (workspace_id, usage_date, requests) VALUES ($1,$2,1)
		ON CONFLICT (workspace_id, usage_date) DO UPDATE SET requests = ai_usage.requests + 1
		WHERE ai_usage.requests < $3 RETURNING requests`, workspaceID, date, limit).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return used, true, nil
}

func (s *Store) ReleaseAIUsage(ctx context.Context, workspaceID id.ID, date string) error {
	_, err := s.pool.Exec(ctx, `UPDATE ai_usage SET requests = GREATEST(requests - 1, 0)
		WHERE workspace_id=$1 AND usage_date=$2`, workspaceID, date)
	return err
}

func (s *Store) AIUsage(ctx context.Context, workspaceID id.ID, date string) (int, error) {
	var used int
	err := s.pool.QueryRow(ctx, `SELECT requests FROM ai_usage WHERE workspace_id=$1 AND usage_date=$2`,
		workspaceID, date).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return used, err
}
