package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) GetQRCodeBySite(ctx context.Context, workspaceID, siteID id.ID) (cafe.Code, error) {
	c, err := scanQR(s.pool.QueryRow(ctx, qrSelect+` WHERE site_id=$1 AND workspace_id=$2`, siteID, workspaceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.Code{}, errx.NotFound("qr_code_not_found", "QR code not found")
	}
	if err != nil {
		return cafe.Code{}, err
	}
	return c, nil
}

func (s *Store) GetQRCodeByToken(ctx context.Context, token string) (cafe.Code, error) {
	c, err := scanQR(s.pool.QueryRow(ctx, qrSelect+` WHERE token=$1`, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.Code{}, errx.NotFound("qr_code_not_found", "QR code not found")
	}
	if err != nil {
		return cafe.Code{}, err
	}
	return c, nil
}

func (s *Store) SaveQRCode(ctx context.Context, c cafe.Code) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO cafe_qr_codes (site_id, workspace_id, token, issued_at, rotated_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (site_id) DO UPDATE SET token=EXCLUDED.token, rotated_at=EXCLUDED.rotated_at`,
		c.SiteID, c.WorkspaceID, c.Token, c.IssuedAt, nullTime(c.RotatedAt))
	return err
}

func (s *Store) ListQRCodes(ctx context.Context, workspaceID id.ID) ([]cafe.Code, error) {
	rows, err := s.pool.Query(ctx, qrSelect+` WHERE workspace_id=$1`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.Code, 0)
	for rows.Next() {
		c, err := scanQR(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetVoice(ctx context.Context, voiceID id.ID) (cafe.Voice, error) {
	v, err := scanVoice(s.pool.QueryRow(ctx, voiceSelect+` WHERE id=$1`, voiceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.Voice{}, errx.NotFound("voice_not_found", "guest voice not found")
	}
	if err != nil {
		return cafe.Voice{}, err
	}
	return v, nil
}

func (s *Store) GetRecoveryByVoice(ctx context.Context, voiceID id.ID) (cafe.Recovery, error) {
	rec, err := scanRecovery(s.pool.QueryRow(ctx, recoverySelect+` WHERE voice_id=$1`, voiceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.Recovery{}, errx.NotFound("recovery_not_found", "recovery not found")
	}
	if err != nil {
		return cafe.Recovery{}, err
	}
	return rec, nil
}

func (s *Store) ListRecoveries(ctx context.Context, workspaceID id.ID) ([]cafe.Recovery, error) {
	rows, err := s.pool.Query(ctx, recoverySelect+` WHERE workspace_id=$1`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.Recovery, 0)
	for rows.Next() {
		rec, err := scanRecovery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) SaveRecovery(ctx context.Context, rec cafe.Recovery) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO cafe_recoveries (
			id, workspace_id, voice_id, status, assignee_user_id, linked_task_id,
			compensation_amount, compensation_currency, compensated_at, closed_at, note, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (voice_id) DO UPDATE SET
			status=EXCLUDED.status, assignee_user_id=EXCLUDED.assignee_user_id, linked_task_id=EXCLUDED.linked_task_id,
			compensation_amount=EXCLUDED.compensation_amount, compensation_currency=EXCLUDED.compensation_currency,
			compensated_at=EXCLUDED.compensated_at, closed_at=EXCLUDED.closed_at, note=EXCLUDED.note, updated_at=EXCLUDED.updated_at`,
		rec.ID, rec.WorkspaceID, rec.VoiceID, rec.Status, uuidOrNil(rec.AssigneeUserID), uuidOrNil(rec.LinkedTaskID),
		rec.CompensationAmount, rec.CompensationCurrency, nullTime(rec.CompensatedAt), nullTime(rec.ClosedAt), rec.Note,
		rec.CreatedAt, rec.UpdatedAt)
	return err
}

func (s *Store) CreateVoice(ctx context.Context, v cafe.Voice) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO cafe_voices (id, workspace_id, site_id, token, kind, rating, comment, contact_name, contact_phone, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		v.ID, v.WorkspaceID, v.SiteID, v.Token, string(v.Kind), v.Rating, v.Comment, v.ContactName, v.ContactPhone, v.CreatedAt)
	return err
}

func (s *Store) ListVoices(ctx context.Context, workspaceID id.ID) ([]cafe.Voice, error) {
	rows, err := s.pool.Query(ctx, voiceSelect+` WHERE workspace_id=$1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cafe.Voice, 0)
	for rows.Next() {
		v, err := scanVoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) CountUnhappyVoicesSince(ctx context.Context, workspaceID id.ID, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM cafe_voices WHERE workspace_id=$1 AND created_at >= $2 AND (kind = 'complaint' OR (rating > 0 AND rating <= 3))`,
		workspaceID, since).Scan(&n)
	return n, err
}

const qrSelect = `SELECT site_id, workspace_id, token, issued_at, coalesce(rotated_at, '0001-01-01') FROM cafe_qr_codes`

func scanQR(row pgx.Row) (cafe.Code, error) {
	var c cafe.Code
	err := row.Scan(&c.SiteID, &c.WorkspaceID, &c.Token, &c.IssuedAt, &c.RotatedAt)
	return c, err
}

const voiceSelect = `SELECT id, workspace_id, site_id, token, kind, rating, comment, contact_name, contact_phone, created_at FROM cafe_voices`

func scanVoice(row pgx.Row) (cafe.Voice, error) {
	var v cafe.Voice
	var kind string
	err := row.Scan(&v.ID, &v.WorkspaceID, &v.SiteID, &v.Token, &kind, &v.Rating, &v.Comment, &v.ContactName, &v.ContactPhone, &v.CreatedAt)
	if err != nil {
		return cafe.Voice{}, err
	}
	v.Kind, _ = cafe.ParseVoiceKind(kind)
	return v, nil
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

const recoverySelect = `SELECT id, workspace_id, voice_id, status,
	coalesce(assignee_user_id, '00000000-0000-0000-0000-000000000000'),
	coalesce(linked_task_id, '00000000-0000-0000-0000-000000000000'),
	compensation_amount, compensation_currency,
	coalesce(compensated_at, '0001-01-01'),
	coalesce(closed_at, '0001-01-01'),
	note, created_at, updated_at FROM cafe_recoveries`

func scanRecovery(row pgx.Row) (cafe.Recovery, error) {
	var rec cafe.Recovery
	err := row.Scan(
		&rec.ID, &rec.WorkspaceID, &rec.VoiceID, &rec.Status, &rec.AssigneeUserID, &rec.LinkedTaskID,
		&rec.CompensationAmount, &rec.CompensationCurrency, &rec.CompensatedAt, &rec.ClosedAt,
		&rec.Note, &rec.CreatedAt, &rec.UpdatedAt,
	)
	return rec, err
}
