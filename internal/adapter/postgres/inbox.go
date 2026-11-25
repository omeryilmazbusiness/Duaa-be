package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/inbox"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) GetConnection(ctx context.Context, workspaceID id.ID) (inbox.Connection, error) {
	var c inbox.Connection
	var provider, status string
	err := s.pool.QueryRow(ctx, `SELECT workspace_id, provider, email, display_name, encrypted_access_token, encrypted_refresh_token,
		client_id, encrypted_client_secret, tenant,
		token_expires_at, scopes, last_sync_at, last_sync_status, last_error, connected_by, created_at, updated_at
		FROM inbox_connections WHERE workspace_id=$1`, workspaceID).
		Scan(&c.WorkspaceID, &provider, &c.Email, &c.DisplayName, &c.EncryptedAccessToken, &c.EncryptedRefreshToken,
			&c.ClientID, &c.EncryptedClientSecret, &c.Tenant,
			&c.TokenExpiresAt, &c.Scopes, &c.LastSyncAt, &status, &c.LastError, &c.ConnectedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return inbox.Connection{}, inbox.ConnectionNotFoundError()
	}
	if err != nil {
		return inbox.Connection{}, err
	}
	c.Provider = inbox.Provider(provider)
	c.LastSyncStatus = inbox.SyncStatus(status)
	return c, nil
}

func (s *Store) SaveConnection(ctx context.Context, c inbox.Connection) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO inbox_connections (
		workspace_id, provider, email, display_name, encrypted_access_token, encrypted_refresh_token,
		client_id, encrypted_client_secret, tenant,
		token_expires_at, scopes, last_sync_at, last_sync_status, last_error, connected_by, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	ON CONFLICT (workspace_id) DO UPDATE SET
		provider=EXCLUDED.provider, email=EXCLUDED.email, display_name=EXCLUDED.display_name,
		encrypted_access_token=EXCLUDED.encrypted_access_token, encrypted_refresh_token=EXCLUDED.encrypted_refresh_token,
		client_id=EXCLUDED.client_id, encrypted_client_secret=EXCLUDED.encrypted_client_secret, tenant=EXCLUDED.tenant,
		token_expires_at=EXCLUDED.token_expires_at, scopes=EXCLUDED.scopes, last_sync_at=EXCLUDED.last_sync_at,
		last_sync_status=EXCLUDED.last_sync_status, last_error=EXCLUDED.last_error, connected_by=EXCLUDED.connected_by,
		updated_at=EXCLUDED.updated_at`,
		c.WorkspaceID, string(c.Provider), c.Email, c.DisplayName, c.EncryptedAccessToken, c.EncryptedRefreshToken,
		c.ClientID, c.EncryptedClientSecret, c.Tenant,
		c.TokenExpiresAt, c.Scopes, c.LastSyncAt, string(c.LastSyncStatus), c.LastError, c.ConnectedBy, c.CreatedAt, c.UpdatedAt)
	return err
}

func (s *Store) DeleteConnection(ctx context.Context, workspaceID id.ID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM inbox_connections WHERE workspace_id=$1`, workspaceID)
	return err
}

func (s *Store) UpsertMessage(ctx context.Context, m inbox.Message) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO inbox_messages (
		id, workspace_id, external_id, conversation_id, folder, from_name, from_email, to_emails, subject, body_text, preview,
		received_at, is_read, has_attachments, category, summary, summary_ar, body_ar, detected_language, enriched_at, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
	ON CONFLICT (workspace_id, external_id) DO UPDATE SET
		conversation_id=EXCLUDED.conversation_id, folder=EXCLUDED.folder, from_name=EXCLUDED.from_name, from_email=EXCLUDED.from_email,
		to_emails=EXCLUDED.to_emails, subject=EXCLUDED.subject, body_text=EXCLUDED.body_text, preview=EXCLUDED.preview,
		received_at=EXCLUDED.received_at, is_read=EXCLUDED.is_read, has_attachments=EXCLUDED.has_attachments,
		category=COALESCE(NULLIF(EXCLUDED.category,''), inbox_messages.category),
		summary=COALESCE(NULLIF(EXCLUDED.summary,''), inbox_messages.summary),
		summary_ar=COALESCE(NULLIF(EXCLUDED.summary_ar,''), inbox_messages.summary_ar),
		body_ar=COALESCE(NULLIF(EXCLUDED.body_ar,''), inbox_messages.body_ar),
		detected_language=COALESCE(NULLIF(EXCLUDED.detected_language,''), inbox_messages.detected_language),
		enriched_at=COALESCE(EXCLUDED.enriched_at, inbox_messages.enriched_at),
		updated_at=EXCLUDED.updated_at`,
		m.ID, m.WorkspaceID, m.ExternalID, m.ConversationID, string(m.Folder), m.FromName, m.FromEmail, m.ToEmails,
		m.Subject, m.BodyText, m.Preview, m.ReceivedAt, m.IsRead, m.HasAttachments, string(m.Category), m.Summary, m.SummaryAR,
		m.BodyAR, m.DetectedLanguage, m.EnrichedAt, m.CreatedAt, m.UpdatedAt)
	return err
}

func (s *Store) GetMessage(ctx context.Context, workspaceID, messageID id.ID) (inbox.Message, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, workspace_id, external_id, conversation_id, folder, from_name, from_email, to_emails,
		subject, body_text, preview, received_at, is_read, has_attachments, category, summary, summary_ar, body_ar, detected_language,
		enriched_at, created_at, updated_at
		FROM inbox_messages WHERE workspace_id=$1 AND id=$2`, workspaceID, messageID)
	return scanMessage(row)
}

func (s *Store) ListMessages(ctx context.Context, workspaceID id.ID, limit int) ([]inbox.Message, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT id, workspace_id, external_id, conversation_id, folder, from_name, from_email, to_emails,
		subject, body_text, preview, received_at, is_read, has_attachments, category, summary, summary_ar, body_ar, detected_language,
		enriched_at, created_at, updated_at
		FROM inbox_messages WHERE workspace_id=$1 ORDER BY received_at DESC LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]inbox.Message, 0)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CountUnread(ctx context.Context, workspaceID id.ID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM inbox_messages WHERE workspace_id=$1 AND is_read=false AND folder='inbox'`, workspaceID).Scan(&n)
	return n, err
}

func (s *Store) DeleteMessages(ctx context.Context, workspaceID id.ID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM inbox_messages WHERE workspace_id=$1`, workspaceID)
	return err
}

type messageScanner interface {
	Scan(dest ...any) error
}

func scanMessage(row messageScanner) (inbox.Message, error) {
	var m inbox.Message
	var folder, category string
	var enriched *time.Time
	err := row.Scan(&m.ID, &m.WorkspaceID, &m.ExternalID, &m.ConversationID, &folder, &m.FromName, &m.FromEmail, &m.ToEmails,
		&m.Subject, &m.BodyText, &m.Preview, &m.ReceivedAt, &m.IsRead, &m.HasAttachments, &category, &m.Summary, &m.SummaryAR, &m.BodyAR,
		&m.DetectedLanguage, &enriched, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return inbox.Message{}, inbox.MessageNotFoundError()
	}
	if err != nil {
		return inbox.Message{}, err
	}
	m.Folder = inbox.Folder(folder)
	m.Category = inbox.Category(category)
	m.EnrichedAt = enriched
	return m, nil
}
