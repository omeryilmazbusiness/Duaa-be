package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const docSelect = `SELECT id, workspace_id, uploaded_by, business_date::text, file_name, source_kind,
	content_sha256, raw_excerpt, status, ai_used, data_coverage_pct, simplified_summary, created_at, updated_at
	FROM workspace_documents`

func (s *Store) GetDocumentByHash(ctx context.Context, workspaceID id.ID, date, sha string) (docintake.Document, error) {
	row := s.pool.QueryRow(ctx, docSelect+` WHERE workspace_id=$1 AND business_date=$2 AND content_sha256=$3`, workspaceID, date, sha)
	d, err := scanDocument(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return docintake.Document{}, errx.NotFound("document_not_found", "document not found")
	}
	return d, err
}

func (s *Store) SaveDocument(ctx context.Context, doc docintake.Document) error {
	raw, err := json.Marshal(doc.Summary)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO workspace_documents (
		id, workspace_id, uploaded_by, business_date, file_name, source_kind, content_sha256,
		raw_excerpt, status, ai_used, data_coverage_pct, simplified_summary, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	ON CONFLICT (workspace_id, business_date, content_sha256) DO UPDATE SET
		status=EXCLUDED.status, ai_used=EXCLUDED.ai_used, data_coverage_pct=EXCLUDED.data_coverage_pct,
		simplified_summary=EXCLUDED.simplified_summary, updated_at=EXCLUDED.updated_at`,
		doc.ID, doc.WorkspaceID, doc.UploadedBy, doc.BusinessDate, doc.FileName, string(doc.SourceKind), doc.ContentSHA256,
		doc.RawExcerpt, string(doc.Status), doc.AIUsed, doc.DataCoveragePct, raw, doc.CreatedAt, doc.UpdatedAt)
	return err
}

func (s *Store) ListDocuments(ctx context.Context, workspaceID id.ID, limit int) ([]docintake.Document, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := s.pool.Query(ctx, docSelect+` WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]docintake.Document, 0)
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func scanDocument(row pgx.Row) (docintake.Document, error) {
	var d docintake.Document
	var kind, status string
	var raw []byte
	err := row.Scan(
		&d.ID, &d.WorkspaceID, &d.UploadedBy, &d.BusinessDate, &d.FileName, &kind,
		&d.ContentSHA256, &d.RawExcerpt, &status, &d.AIUsed, &d.DataCoveragePct, &raw, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return docintake.Document{}, err
	}
	d.SourceKind = docintake.SourceKind(kind)
	d.Status = docintake.Status(status)
	_ = json.Unmarshal(raw, &d.Summary)
	return d, nil
}
