package cafeqr

import (
	"context"
	"errors"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// IssueTable records a seat and mints a token once. The same number returns the same code.
func (s *Service) IssueTable(ctx context.Context, ac actor.Context, tableNumber string) (CodeView, error) {
	if err := requireCafe(ac); err != nil {
		return CodeView{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageWorkspace) {
		return CodeView{}, errx.Forbidden("forbidden", "only the general manager can issue a table QR")
	}
	number, err := cafe.ParseTableNumber(tableNumber)
	if err != nil {
		return CodeView{}, err
	}
	def, err := s.mutableDefinition(ctx, ac)
	if err != nil {
		return CodeView{}, err
	}
	if site, ok := def.TableSite(number); ok {
		code, err := s.ensure(ctx, ac.Workspace.ID, site)
		if err != nil {
			return CodeView{}, err
		}
		return s.toCodeView(def, site, code), nil
	}
	site := cafe.QRSite{ID: id.New(), Label: number, Area: cafe.AreaTable, Active: true}
	if bid := firstActiveBranch(def); bid != id.Zero() {
		site.BranchID = bid
	}
	def.QRSites = append(def.QRSites, site)
	if err := s.persistDefinition(ctx, ac, def); err != nil {
		return CodeView{}, err
	}
	code, err := s.ensure(ctx, ac.Workspace.ID, site)
	if err != nil {
		return CodeView{}, err
	}
	return s.toCodeView(def, site, code), nil
}

// RenameTable changes the printed number only. Token, path and image stay.
func (s *Service) RenameTable(ctx context.Context, ac actor.Context, siteID, tableNumber string) (CodeView, error) {
	if err := requireCafe(ac); err != nil {
		return CodeView{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageWorkspace) {
		return CodeView{}, errx.Forbidden("forbidden", "only the general manager can rename a table")
	}
	sid, err := id.Parse(siteID)
	if err != nil {
		return CodeView{}, errx.Invalid("invalid_qr_site", "QR location is invalid")
	}
	number, err := cafe.ParseTableNumber(tableNumber)
	if err != nil {
		return CodeView{}, err
	}
	def, err := s.mutableDefinition(ctx, ac)
	if err != nil {
		return CodeView{}, err
	}
	idx := -1
	for i, q := range def.QRSites {
		if q.ID == sid {
			idx = i
			break
		}
	}
	if idx < 0 {
		return CodeView{}, errx.NotFound("qr_site_not_found", "QR location not found")
	}
	if other, ok := def.TableSite(number); ok && other.ID != sid {
		return CodeView{}, errx.Invalid("duplicate_table", "each table number is used once")
	}
	def.QRSites[idx].Label = number
	if def.QRSites[idx].Area == "" {
		def.QRSites[idx].Area = cafe.AreaTable
	}
	if err := s.persistDefinition(ctx, ac, def); err != nil {
		return CodeView{}, err
	}
	site := def.QRSites[idx]
	code, err := s.ensure(ctx, ac.Workspace.ID, site)
	if err != nil {
		return CodeView{}, err
	}
	return s.toCodeView(def, site, code), nil
}

func (s *Service) mutableDefinition(ctx context.Context, ac actor.Context) (cafe.Definition, error) {
	def, err := s.defs.GetCafeDefinition(ctx, ac.Workspace.ID)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return cafe.Seed(ac.Workspace.ID, ac.Workspace.Name, ac.Workspace.Timezone), nil
		}
		return cafe.Definition{}, err
	}
	return def.Clone(), nil
}

func (s *Service) persistDefinition(ctx context.Context, ac actor.Context, def cafe.Definition) error {
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return errx.Invalid("invalid_actor", "actor is required")
	}
	def.WorkspaceID = ac.Workspace.ID
	def.UpdatedAt = s.clock.Now().UTC()
	def.UpdatedBy = uid
	def.Normalize()
	if err := def.Validate(); err != nil {
		return err
	}
	return s.defs.SaveCafeDefinition(ctx, def)
}

func firstActiveBranch(def cafe.Definition) id.ID {
	for _, b := range def.Branches {
		if b.Active {
			return b.ID
		}
	}
	if len(def.Branches) == 1 {
		return def.Branches[0].ID
	}
	return id.Zero()
}
