package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/adapter/cafedoc"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) GetCafeDefinition(ctx context.Context, workspaceID id.ID) (cafe.Definition, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT document FROM cafe_definitions WHERE workspace_id=$1`, workspaceID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "cafe definition not found")
	}
	if err != nil {
		return cafe.Definition{}, err
	}
	return cafedoc.Unmarshal(raw)
}

func (s *Store) SaveCafeDefinition(ctx context.Context, def cafe.Definition) error {
	raw, err := cafedoc.Marshal(def)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO cafe_definitions (workspace_id, document, updated_at, updated_by_user_id)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (workspace_id) DO UPDATE SET document=EXCLUDED.document, updated_at=EXCLUDED.updated_at,
		updated_by_user_id=EXCLUDED.updated_by_user_id`,
		def.WorkspaceID, raw, def.UpdatedAt, uuidOrNil(def.UpdatedBy))
	return err
}

func (s *Store) UpdateWorkspace(ctx context.Context, w tenancy.Workspace) error {
	cmd, err := s.pool.Exec(ctx, `UPDATE workspaces SET name=$2, timezone=$3, updated_at=$4 WHERE id=$1`,
		w.ID, w.Name, w.Timezone, w.UpdatedAt)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errx.NotFound("workspace_not_found", "workspace not found")
	}
	return nil
}
