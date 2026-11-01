package cafedef

import (
	"context"
	"errors"
	"strings"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	defs   Repository
	spaces WorkspaceWriter
	clock  clock.Clock
}

func New(defs Repository, spaces WorkspaceWriter, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{defs: defs, spaces: spaces, clock: clk}
}

type View struct {
	Definition   cafe.Definition   `json:"-"`
	Completeness cafe.Completeness `json:"completeness"`
	CanWrite     bool              `json:"canWrite"`
	Persisted    bool              `json:"persisted"`
}

type SaveInput struct {
	DisplayName string
	Currency    string
	Timezone    string
	POSVendor   string
	POSStatus   string
	Branches    []cafe.Branch
	QRSites     []cafe.QRSite
	Shifts      []cafe.Shift
	SLA         cafe.SLA
}

func (s *Service) Get(ctx context.Context, ac actor.Context) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ViewCafe) {
		return View{}, errx.Forbidden("forbidden", "you cannot view the cafe definition")
	}
	cur, err := s.defs.GetCafeDefinition(ctx, ac.Workspace.ID)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return seedView(ac), nil
		}
		return View{}, err
	}
	return viewOf(cur, ac, true), nil
}

func (s *Service) Save(ctx context.Context, ac actor.Context, in SaveInput) (View, error) {
	if err := requireCafe(ac); err != nil {
		return View{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageWorkspace) {
		return View{}, errx.Forbidden("forbidden", "only the general manager can edit the cafe definition")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return View{}, errx.Invalid("invalid_actor", "actor is required")
	}
	now := s.clock.Now().UTC()
	def := cafe.Definition{
		WorkspaceID: ac.Workspace.ID,
		DisplayName: in.DisplayName,
		Currency:    in.Currency,
		Timezone:    in.Timezone,
		POSVendor:   in.POSVendor,
		POSStatus:   cafe.POSStatus(in.POSStatus),
		Branches:    in.Branches,
		QRSites:     in.QRSites,
		Shifts:      in.Shifts,
		SLA:         in.SLA,
		UpdatedAt:   now,
		UpdatedBy:   uid,
	}
	if existing, err := s.defs.GetCafeDefinition(ctx, ac.Workspace.ID); err == nil {
		def = mergeIDs(existing, def)
	} else if !errors.Is(err, errx.ErrNotFound) {
		return View{}, err
	}
	def.Normalize()
	if err := def.Validate(); err != nil {
		return View{}, err
	}
	if err := s.defs.SaveCafeDefinition(ctx, def); err != nil {
		return View{}, err
	}
	if err := s.syncWorkspace(ctx, ac.Workspace, def); err != nil {
		return View{}, err
	}
	return viewOf(def, ac, true), nil
}

func (s *Service) syncWorkspace(ctx context.Context, ws tenancy.Workspace, def cafe.Definition) error {
	name := strings.TrimSpace(def.DisplayName)
	tz := strings.TrimSpace(def.Timezone)
	if name == "" && tz == "" {
		return nil
	}
	if name == ws.Name && (tz == "" || tz == ws.Timezone) {
		return nil
	}
	cur, err := s.spaces.GetWorkspace(ctx, ws.ID)
	if err != nil {
		return err
	}
	if name != "" {
		cur.Name = name
	}
	if tz != "" {
		cur.Timezone = tz
	}
	cur.UpdatedAt = def.UpdatedAt
	return s.spaces.UpdateWorkspace(ctx, cur)
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "cafe definition is only available on a cafe workspace")
	}
	return nil
}

func seedView(ac actor.Context) View {
	return viewOf(cafe.Seed(ac.Workspace.ID, ac.Workspace.Name, ac.Workspace.Timezone), ac, false)
}

func viewOf(def cafe.Definition, ac actor.Context, persisted bool) View {
	return View{
		Definition:   def.Clone(),
		Completeness: def.Completeness(),
		CanWrite:     rbac.Require(ac.Scope, rbac.ManageWorkspace),
		Persisted:    persisted,
	}
}

func mergeIDs(prev, next cafe.Definition) cafe.Definition {
	next.WorkspaceID = prev.WorkspaceID
	if next.UpdatedBy == id.Zero() {
		next.UpdatedBy = prev.UpdatedBy
	}
	return next
}
