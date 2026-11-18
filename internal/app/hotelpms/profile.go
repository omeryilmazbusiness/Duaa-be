package hotelpms

import (
	"context"
	"errors"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type ProfileView struct {
	DisplayName string `json:"displayName"`
	TotalRooms  int    `json:"totalRooms"`
	Currency    string `json:"currency"`
	Timezone    string `json:"timezone"`
	StarRating  int    `json:"starRating"`
	City        string `json:"city"`
	Address     string `json:"address"`
	Phone       string `json:"phone"`
	GMName      string `json:"gmName"`
	Notes       string `json:"notes"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	Persisted   bool   `json:"persisted"`
	CanWrite    bool   `json:"canWrite"`
}

type SaveProfileInput struct {
	DisplayName string
	TotalRooms  int
	Currency    string
	Timezone    string
	StarRating  int
	City        string
	Address     string
	Phone       string
	GMName      string
	Notes       string
}

// WithWorkspaces lets profile saves rename the workspace; without it only the profile row changes.
func (s *Service) WithWorkspaces(w WorkspaceWriter) *Service {
	s.spaces = w
	return s
}

func (s *Service) GetProfile(ctx context.Context, ac actor.Context) (ProfileView, error) {
	if err := requireHotel(ac); err != nil {
		return ProfileView{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.ViewDashboard) {
		return ProfileView{}, errx.Forbidden("forbidden", "cannot view hotel profile")
	}
	recent, err := s.repo.ListPMSSnapshots(ctx, ac.Workspace.ID, 1)
	if err != nil {
		return ProfileView{}, err
	}
	return s.profileView(ctx, ac, recent)
}

func (s *Service) SaveProfile(ctx context.Context, ac actor.Context, in SaveProfileInput) (ProfileView, error) {
	if err := requireHotel(ac); err != nil {
		return ProfileView{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.ManageWorkspace) {
		return ProfileView{}, errx.Forbidden("forbidden", "only the general manager can edit hotel information")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return ProfileView{}, errx.Unauthenticated("invalid token")
	}
	now := s.clock.Now().UTC()
	p := hotel.Profile{
		WorkspaceID: ac.Workspace.ID,
		DisplayName: in.DisplayName,
		TotalRooms:  in.TotalRooms,
		Currency:    in.Currency,
		Timezone:    in.Timezone,
		StarRating:  in.StarRating,
		City:        in.City,
		Address:     in.Address,
		Phone:       in.Phone,
		GMName:      in.GMName,
		Notes:       in.Notes,
		UpdatedAt:   now,
		UpdatedBy:   uid,
	}
	p.Normalize()
	if p.Timezone == "" {
		p.Timezone = ac.Workspace.Timezone
	}
	if err := p.Validate(); err != nil {
		return ProfileView{}, err
	}
	if err := s.repo.SaveHotelProfile(ctx, p); err != nil {
		return ProfileView{}, err
	}
	if err := s.syncWorkspace(ctx, ac, p); err != nil {
		return ProfileView{}, err
	}
	return toProfileView(p, true, true), nil
}

func (s *Service) profileView(ctx context.Context, ac actor.Context, recent []hotel.PMSSnapshot) (ProfileView, error) {
	canWrite := rbac.Allows(ac.Scope, rbac.ManageWorkspace)
	p, err := s.repo.GetHotelProfile(ctx, ac.Workspace.ID)
	if err == nil {
		return toProfileView(p, true, canWrite), nil
	}
	if !errors.Is(err, errx.ErrNotFound) {
		return ProfileView{}, err
	}
	seed := hotel.Profile{
		DisplayName: ac.Workspace.Name,
		Currency:    "USD",
		Timezone:    ac.Workspace.Timezone,
	}
	if len(recent) > 0 {
		seed.TotalRooms = recent[0].TotalRooms
		if recent[0].Currency != "" {
			seed.Currency = recent[0].Currency
		}
	}
	return toProfileView(seed, false, canWrite), nil
}

func (s *Service) syncWorkspace(ctx context.Context, ac actor.Context, p hotel.Profile) error {
	if s.spaces == nil {
		return nil
	}
	if p.DisplayName == ac.Workspace.Name && p.Timezone == ac.Workspace.Timezone {
		return nil
	}
	cur, err := s.spaces.GetWorkspace(ctx, ac.Workspace.ID)
	if err != nil {
		return err
	}
	cur.Name = p.DisplayName
	if p.Timezone != "" {
		cur.Timezone = p.Timezone
	}
	cur.UpdatedAt = p.UpdatedAt
	return s.spaces.UpdateWorkspace(ctx, cur)
}

func toProfileView(p hotel.Profile, persisted, canWrite bool) ProfileView {
	v := ProfileView{
		DisplayName: p.DisplayName,
		TotalRooms:  p.TotalRooms,
		Currency:    p.Currency,
		Timezone:    p.Timezone,
		StarRating:  p.StarRating,
		City:        p.City,
		Address:     p.Address,
		Phone:       p.Phone,
		GMName:      p.GMName,
		Notes:       p.Notes,
		Persisted:   persisted,
		CanWrite:    canWrite,
	}
	if !p.UpdatedAt.IsZero() {
		v.UpdatedAt = p.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return v
}
