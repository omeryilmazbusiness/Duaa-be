package hotelpms

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	GetOHIPConfig(ctx context.Context, workspaceID id.ID) (hotel.OHIPConfig, error)
	SaveOHIPConfig(ctx context.Context, cfg hotel.OHIPConfig) error
	GetPMSSnapshot(ctx context.Context, workspaceID id.ID, businessDate string) (hotel.PMSSnapshot, error)
	SavePMSSnapshot(ctx context.Context, snap hotel.PMSSnapshot) error
	ListPMSSnapshots(ctx context.Context, workspaceID id.ID, limit int) ([]hotel.PMSSnapshot, error)
	GetHotelProfile(ctx context.Context, workspaceID id.ID) (hotel.Profile, error)
	SaveHotelProfile(ctx context.Context, p hotel.Profile) error
}

// WorkspaceWriter keeps the workspace row's name and timezone in step with the hotel profile.
type WorkspaceWriter interface {
	GetWorkspace(ctx context.Context, workspaceID id.ID) (tenancy.Workspace, error)
	UpdateWorkspace(ctx context.Context, w tenancy.Workspace) error
}

type Cipher interface {
	Seal(plaintext, aad string) (string, error)
	Open(ciphertext, aad string) (string, error)
}

type OHIPFetchRequest struct {
	HostURL      string
	HotelID      string
	ClientID     string
	ClientSecret string
	AppKey       string
	BusinessDate string
	RecorderID   id.ID
	WorkspaceID  id.ID
}

type OHIPClient interface {
	FetchDay(ctx context.Context, req OHIPFetchRequest) (hotel.PMSSnapshot, error)
}
