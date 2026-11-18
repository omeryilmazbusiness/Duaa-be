package hotelpms

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	repo   Repository
	cipher Cipher
	ohip   OHIPClient
	spaces WorkspaceWriter
	clock  clock.Clock
}

func New(repo Repository, cipher Cipher, ohip OHIPClient, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, cipher: cipher, ohip: ohip, clock: clk}
}

type OHIPConfigView struct {
	Configured     bool   `json:"configured"`
	Enabled        bool   `json:"enabled"`
	HostURL        string `json:"hostUrl,omitempty"`
	HotelID        string `json:"hotelId,omitempty"`
	ClientID       string `json:"clientId,omitempty"`
	SecretHint     string `json:"secretHint,omitempty"`
	AppKeyHint     string `json:"appKeyHint,omitempty"`
	SyncMode       string `json:"syncMode"`
	LastSyncAt     string `json:"lastSyncAt,omitempty"`
	LastSyncStatus string `json:"lastSyncStatus"`
	LastError      string `json:"lastError,omitempty"`
}

type SnapshotView struct {
	ID                string                      `json:"id"`
	BusinessDate      string                      `json:"businessDate"`
	Source            string                      `json:"source"`
	ExternalBatchRef  string                      `json:"externalBatchRef,omitempty"`
	Currency          string                      `json:"currency"`
	TotalRooms        int                         `json:"totalRooms"`
	AvailableRooms    int                         `json:"availableRooms"`
	OccupiedRooms     int                         `json:"occupiedRooms"`
	OutOfOrderRooms   int                         `json:"outOfOrderRooms"`
	OccupancyPct      float64                     `json:"occupancyPct"`
	RoomRevenueMinor  int64                       `json:"roomRevenueMinor"`
	FBRevenueMinor    int64                       `json:"fbRevenueMinor"`
	OtherRevenueMinor int64                       `json:"otherRevenueMinor"`
	TotalRevenueMinor int64                       `json:"totalRevenueMinor"`
	ADRMinor          int64                       `json:"adrMinor"`
	RevPARMinor       int64                       `json:"revparMinor"`
	ArrivalsCount     int                         `json:"arrivalsCount"`
	DeparturesCount   int                         `json:"departuresCount"`
	InHouseGuests     int                         `json:"inHouseGuests"`
	ReservationsCount int                         `json:"reservationsCount"`
	CancelledCount    int                         `json:"cancelledCount"`
	NoShowCount       int                         `json:"noShowCount"`
	Readiness         []hotel.DepartmentReadiness `json:"readiness"`
	ReadinessPct      int                         `json:"readinessPct"`
	Reservations      []hotel.ReservationRow      `json:"reservations"`
	CapturedAt        string                      `json:"capturedAt"`
}

type SnapshotSummary struct {
	BusinessDate      string  `json:"businessDate"`
	Currency          string  `json:"currency"`
	OccupancyPct      float64 `json:"occupancyPct"`
	TotalRevenueMinor int64   `json:"totalRevenueMinor"`
	RoomRevenueMinor  int64   `json:"roomRevenueMinor"`
	ADRMinor          int64   `json:"adrMinor"`
	RevPARMinor       int64   `json:"revparMinor"`
	ReservationsCount int     `json:"reservationsCount"`
	Source            string  `json:"source"`
}

type ComparisonView struct {
	YesterdayDate         string   `json:"yesterdayDate"`
	HasYesterday          bool     `json:"hasYesterday"`
	YesterdayRevenueMinor int64    `json:"yesterdayRevenueMinor"`
	RevenueDeltaMinor     int64    `json:"revenueDeltaMinor"`
	RevenueDeltaPct       *float64 `json:"revenueDeltaPct,omitempty"`
	YesterdayOccupancyPct *float64 `json:"yesterdayOccupancyPct,omitempty"`
	OccupancyDeltaPts     *float64 `json:"occupancyDeltaPts,omitempty"`
}

type DashboardView struct {
	WorkspaceID   string            `json:"workspaceId"`
	WorkspaceName string            `json:"workspaceName"`
	BusinessDate  string            `json:"businessDate"`
	Timezone      string            `json:"timezone"`
	Freshness     string            `json:"freshness"`
	ReasonKey     string            `json:"reasonKey,omitempty"`
	Replayed      bool              `json:"replayed,omitempty"`
	Connector     OHIPConfigView    `json:"connector"`
	Today         *SnapshotView     `json:"today,omitempty"`
	Comparison    *ComparisonView   `json:"comparison,omitempty"`
	RecentDays    []SnapshotSummary `json:"recentDays"`
	CanManageOHIP bool              `json:"canManageOhip"`
	CanRecord     bool              `json:"canRecord"`
	Profile       ProfileView       `json:"profile"`
}

type SaveOHIPInput struct {
	Enabled      bool
	HostURL      string
	HotelID      string
	ClientID     string
	ClientSecret string
	AppKey       string
	SyncMode     string
}

type SnapshotInput struct {
	BusinessDate      string
	Source            string
	ExternalBatchRef  string
	Currency          string
	TotalRooms        int
	OccupiedRooms     int
	OutOfOrderRooms   int
	RoomRevenueMinor  int64
	FBRevenueMinor    int64
	OtherRevenueMinor int64
	ArrivalsCount     int
	DeparturesCount   int
	InHouseGuests     int
	ReservationsCount int
	CancelledCount    int
	NoShowCount       int
	Readiness         []hotel.DepartmentReadiness
	Reservations      []hotel.ReservationRow
}

func (s *Service) GetDashboard(ctx context.Context, ac actor.Context, date string) (DashboardView, error) {
	if err := requireHotel(ac); err != nil {
		return DashboardView{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.ViewDashboard) {
		return DashboardView{}, errx.Forbidden("forbidden", "cannot view hotel dashboard")
	}
	day, err := s.day(ac.Workspace.Timezone, date)
	if err != nil {
		return DashboardView{}, err
	}
	return s.buildView(ctx, ac, day, false)
}

func (s *Service) SaveOHIPConfig(ctx context.Context, ac actor.Context, in SaveOHIPInput) (OHIPConfigView, error) {
	if err := requireHotel(ac); err != nil {
		return OHIPConfigView{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.ManageWorkspace) {
		return OHIPConfigView{}, errx.Forbidden("forbidden", "only the general manager can configure OHIP")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return OHIPConfigView{}, errx.Unauthenticated("invalid token")
	}
	mode, err := hotel.ParseSyncMode(in.SyncMode)
	if err != nil {
		return OHIPConfigView{}, err
	}
	hostURL, err := hotel.NormalizeHostURL(in.HostURL)
	if err != nil {
		return OHIPConfigView{}, err
	}
	hotelID := strings.TrimSpace(in.HotelID)
	clientID := strings.TrimSpace(in.ClientID)
	if mode == hotel.SyncModeOHIP && in.Enabled {
		if hostURL == "" || hotelID == "" || clientID == "" {
			return OHIPConfigView{}, errx.Invalid("ohip_fields_required", "OHIP hostUrl, hotelId and clientId are required")
		}
	}

	now := s.clock.Now().UTC()
	cur, err := s.repo.GetOHIPConfig(ctx, ac.Workspace.ID)
	hasCur := err == nil
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return OHIPConfigView{}, err
	}
	next := hotel.OHIPConfig{
		WorkspaceID:    ac.Workspace.ID,
		Enabled:        in.Enabled,
		HostURL:        hostURL,
		HotelID:        hotelID,
		ClientID:       clientID,
		SyncMode:       mode,
		LastSyncStatus: hotel.SyncNever,
		UpdatedBy:      uid,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if hasCur {
		next.CreatedAt = cur.CreatedAt
		next.EncryptedClientSecret = cur.EncryptedClientSecret
		next.SecretHint = cur.SecretHint
		next.EncryptedAppKey = cur.EncryptedAppKey
		next.AppKeyHint = cur.AppKeyHint
		next.LastSyncAt = cur.LastSyncAt
		next.LastSyncStatus = cur.LastSyncStatus
		next.LastError = cur.LastError
	}
	if secret := strings.TrimSpace(in.ClientSecret); secret != "" {
		if s.cipher == nil {
			return OHIPConfigView{}, errx.Invalid("ohip_cipher_missing", "secret vault unavailable")
		}
		sealed, err := s.cipher.Seal(secret, ac.Workspace.ID.String())
		if err != nil {
			return OHIPConfigView{}, err
		}
		next.EncryptedClientSecret = sealed
		next.SecretHint = hotel.KeyHint(secret)
	}
	if appKey := strings.TrimSpace(in.AppKey); appKey != "" {
		if s.cipher == nil {
			return OHIPConfigView{}, errx.Invalid("ohip_cipher_missing", "secret vault unavailable")
		}
		sealed, err := s.cipher.Seal(appKey, ac.Workspace.ID.String())
		if err != nil {
			return OHIPConfigView{}, err
		}
		next.EncryptedAppKey = sealed
		next.AppKeyHint = hotel.KeyHint(appKey)
	}
	if mode == hotel.SyncModeOHIP && in.Enabled && (next.EncryptedClientSecret == "" || next.EncryptedAppKey == "") {
		return OHIPConfigView{}, errx.Invalid("ohip_secrets_required", "OHIP clientSecret and appKey are required")
	}
	if err := s.repo.SaveOHIPConfig(ctx, next); err != nil {
		return OHIPConfigView{}, err
	}
	return toConfigView(next), nil
}

func (s *Service) SyncOHIP(ctx context.Context, ac actor.Context, date string) (DashboardView, error) {
	if err := requireHotel(ac); err != nil {
		return DashboardView{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.RecordHotelPMS) {
		return DashboardView{}, errx.Forbidden("forbidden", "cannot sync hotel OHIP")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return DashboardView{}, errx.Unauthenticated("invalid token")
	}
	day, err := s.day(ac.Workspace.Timezone, date)
	if err != nil {
		return DashboardView{}, err
	}
	cfg, err := s.repo.GetOHIPConfig(ctx, ac.Workspace.ID)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return DashboardView{}, errx.Conflict("ohip_not_configured", "OHIP connector is not configured yet")
		}
		return DashboardView{}, err
	}
	if !cfg.Enabled || cfg.SyncMode != hotel.SyncModeOHIP || s.ohip == nil {
		return DashboardView{}, errx.Conflict("ohip_not_enabled", "live OHIP sync is not enabled")
	}
	secret, err := s.cipher.Open(cfg.EncryptedClientSecret, ac.Workspace.ID.String())
	if err != nil {
		return DashboardView{}, errx.Conflict("ohip_secret_unreadable", "stored OHIP secret cannot be read")
	}
	appKey, err := s.cipher.Open(cfg.EncryptedAppKey, ac.Workspace.ID.String())
	if err != nil {
		return DashboardView{}, errx.Conflict("ohip_secret_unreadable", "stored OHIP app key cannot be read")
	}
	now := s.clock.Now().UTC()
	snap, syncErr := s.ohip.FetchDay(ctx, OHIPFetchRequest{
		HostURL: cfg.HostURL, HotelID: cfg.HotelID, ClientID: cfg.ClientID,
		ClientSecret: secret, AppKey: appKey, BusinessDate: day,
		RecorderID: uid, WorkspaceID: ac.Workspace.ID,
	})
	if syncErr != nil {
		cfg.LastSyncAt = &now
		cfg.LastSyncStatus = hotel.SyncError
		cfg.LastError = sanitizeSyncErr(syncErr)
		cfg.UpdatedAt = now
		_ = s.repo.SaveOHIPConfig(ctx, cfg)
		// PDF AT-10: retain last valid snapshot with stale freshness; never zero it out.
		return s.buildView(ctx, ac, day, false)
	}
	snap.WorkspaceID = ac.Workspace.ID
	snap.RecordedBy = uid
	snap.BusinessDate = day
	snap.Source = hotel.SourceOHIPLive
	if snap.CapturedAt.IsZero() {
		snap.CapturedAt = now
	}
	snap.CreatedAt = now
	snap.UpdatedAt = now
	if err := snap.NormalizeAndValidate(); err != nil {
		return DashboardView{}, err
	}
	if err := s.repo.SavePMSSnapshot(ctx, snap); err != nil {
		return DashboardView{}, err
	}
	cfg.LastSyncAt = &now
	cfg.LastSyncStatus = hotel.SyncOK
	cfg.LastError = ""
	cfg.UpdatedAt = now
	_ = s.repo.SaveOHIPConfig(ctx, cfg)
	return s.buildView(ctx, ac, day, false)
}

func (s *Service) ImportSnapshot(ctx context.Context, ac actor.Context, in SnapshotInput) (DashboardView, error) {
	if err := requireHotel(ac); err != nil {
		return DashboardView{}, err
	}
	if !rbac.Allows(ac.Scope, rbac.RecordHotelPMS) {
		return DashboardView{}, errx.Forbidden("forbidden", "cannot record hotel PMS snapshot")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return DashboardView{}, errx.Unauthenticated("invalid token")
	}
	day, err := s.day(ac.Workspace.Timezone, in.BusinessDate)
	if err != nil {
		return DashboardView{}, err
	}
	now := s.clock.Now().UTC()
	replayed := false
	if ref := strings.TrimSpace(in.ExternalBatchRef); ref != "" {
		if existing, err := s.repo.GetPMSSnapshot(ctx, ac.Workspace.ID, day); err == nil && existing.ExternalBatchRef == ref {
			replayed = true
			return s.buildView(ctx, ac, day, replayed)
		}
	}
	snap := hotel.PMSSnapshot{
		WorkspaceID:       ac.Workspace.ID,
		BusinessDate:      day,
		Source:            hotel.PMSSource(in.Source),
		ExternalBatchRef:  in.ExternalBatchRef,
		Currency:          in.Currency,
		TotalRooms:        in.TotalRooms,
		OccupiedRooms:     in.OccupiedRooms,
		OutOfOrderRooms:   in.OutOfOrderRooms,
		RoomRevenueMinor:  in.RoomRevenueMinor,
		FBRevenueMinor:    in.FBRevenueMinor,
		OtherRevenueMinor: in.OtherRevenueMinor,
		ArrivalsCount:     in.ArrivalsCount,
		DeparturesCount:   in.DeparturesCount,
		InHouseGuests:     in.InHouseGuests,
		ReservationsCount: in.ReservationsCount,
		CancelledCount:    in.CancelledCount,
		NoShowCount:       in.NoShowCount,
		Readiness:         in.Readiness,
		Reservations:      in.Reservations,
		CapturedAt:        now,
		RecordedBy:        uid,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := snap.NormalizeAndValidate(); err != nil {
		return DashboardView{}, err
	}
	if err := s.repo.SavePMSSnapshot(ctx, snap); err != nil {
		return DashboardView{}, err
	}
	return s.buildView(ctx, ac, day, replayed)
}

// ImportOPERAFile parses JSON or CSV exported from OPERA / Sheraton reports into a canonical snapshot.
func (s *Service) ImportOPERAFile(ctx context.Context, ac actor.Context, businessDate, externalBatchRef, rawContent string) (DashboardView, error) {
	if strings.TrimSpace(rawContent) == "" {
		return DashboardView{}, errx.Invalid("empty_opera_export", "OPERA export payload is empty")
	}
	in, err := ParseOPERAExport(rawContent, businessDate, externalBatchRef)
	if err != nil {
		return DashboardView{}, err
	}
	in.Source = string(hotel.SourceOPERAExport)
	return s.ImportSnapshot(ctx, ac, in)
}

func (s *Service) buildView(ctx context.Context, ac actor.Context, day string, replayed bool) (DashboardView, error) {
	cfg, cfgErr := s.repo.GetOHIPConfig(ctx, ac.Workspace.ID)
	if cfgErr != nil && !errors.Is(cfgErr, errx.ErrNotFound) {
		return DashboardView{}, cfgErr
	}
	var todaySnap *hotel.PMSSnapshot
	if snap, err := s.repo.GetPMSSnapshot(ctx, ac.Workspace.ID, day); err == nil {
		todaySnap = &snap
	} else if !errors.Is(err, errx.ErrNotFound) {
		return DashboardView{}, err
	}

	yday := previousDate(day)
	var ydaySnap *hotel.PMSSnapshot
	if snap, err := s.repo.GetPMSSnapshot(ctx, ac.Workspace.ID, yday); err == nil {
		ydaySnap = &snap
	} else if !errors.Is(err, errx.ErrNotFound) {
		return DashboardView{}, err
	}

	recent, err := s.repo.ListPMSSnapshots(ctx, ac.Workspace.ID, 7)
	if err != nil {
		return DashboardView{}, err
	}
	summaries := make([]SnapshotSummary, 0, len(recent))
	for _, r := range recent {
		summaries = append(summaries, SnapshotSummary{
			BusinessDate:      r.BusinessDate,
			Currency:          r.Currency,
			OccupancyPct:      bpsToPct(r.OccupancyBasisPoints()),
			TotalRevenueMinor: r.TotalRevenueMinor(),
			RoomRevenueMinor:  r.RoomRevenueMinor,
			ADRMinor:          r.ADRMinor(),
			RevPARMinor:       r.RevPARMinor(),
			ReservationsCount: r.ReservationsCount,
			Source:            string(r.Source),
		})
	}

	now := s.clock.Now().UTC()
	fresh := hotel.ComputeFreshness(now, todaySnap, cfg.LastSyncStatus)
	out := DashboardView{
		WorkspaceID:   ac.Workspace.ID.String(),
		WorkspaceName: ac.Workspace.Name,
		BusinessDate:  day,
		Timezone:      ac.Workspace.Timezone,
		Freshness:     string(fresh),
		Replayed:      replayed,
		Connector:     toConfigView(cfg),
		RecentDays:    summaries,
		CanManageOHIP: rbac.Allows(ac.Scope, rbac.ManageWorkspace),
		CanRecord:     rbac.Allows(ac.Scope, rbac.RecordHotelPMS),
	}
	profile, err := s.profileView(ctx, ac, recent)
	if err != nil {
		return DashboardView{}, err
	}
	out.Profile = profile
	if todaySnap == nil {
		out.ReasonKey = "hotel.pmsNotConnected"
		return out, nil
	}
	tv := toSnapshotView(*todaySnap)
	out.Today = &tv
	comp := ComparisonView{YesterdayDate: yday}
	if ydaySnap != nil && ydaySnap.Currency == todaySnap.Currency {
		comp.HasYesterday = true
		yRev := ydaySnap.TotalRevenueMinor()
		tRev := todaySnap.TotalRevenueMinor()
		comp.YesterdayRevenueMinor = yRev
		comp.RevenueDeltaMinor = tRev - yRev
		if yRev > 0 {
			pct := round1(float64(tRev-yRev) * 100 / float64(yRev))
			comp.RevenueDeltaPct = &pct
		}
		yOcc := bpsToPct(ydaySnap.OccupancyBasisPoints())
		tOcc := bpsToPct(todaySnap.OccupancyBasisPoints())
		occDelta := round1(tOcc - yOcc)
		comp.YesterdayOccupancyPct = &yOcc
		comp.OccupancyDeltaPts = &occDelta
	}
	out.Comparison = &comp
	return out, nil
}

func ParseOPERAExport(raw, fallbackDate, batchRef string) (SnapshotInput, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") {
		var in SnapshotInput
		if err := json.Unmarshal([]byte(trimmed), &in); err != nil {
			return SnapshotInput{}, errx.Invalid("invalid_opera_json", "OPERA JSON could not be parsed")
		}
		if in.BusinessDate == "" {
			in.BusinessDate = fallbackDate
		}
		if in.ExternalBatchRef == "" {
			in.ExternalBatchRef = batchRef
		}
		return in, nil
	}
	r := csv.NewReader(bytes.NewBufferString(trimmed))
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil && !errors.Is(err, io.EOF) {
		return SnapshotInput{}, errx.Invalid("invalid_opera_csv", "OPERA CSV could not be parsed")
	}
	if len(rows) < 2 {
		return SnapshotInput{}, errx.Invalid("invalid_opera_csv", "OPERA CSV needs header and at least one row")
	}
	headers := make([]string, len(rows[0]))
	for i, h := range rows[0] {
		headers[i] = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(h, " ", "_")))
	}
	idx := map[string]int{}
	for i, h := range headers {
		idx[h] = i
	}
	first := rows[1]
	get := func(keys ...string) string {
		for _, k := range keys {
			if i, ok := idx[k]; ok && i < len(first) {
				return strings.TrimSpace(first[i])
			}
		}
		return ""
	}
	in := SnapshotInput{
		BusinessDate:      firstNonEmpty(get("business_date", "businessdate", "date"), fallbackDate),
		ExternalBatchRef:  batchRef,
		Currency:          firstNonEmpty(get("currency", "curr"), "USD"),
		TotalRooms:        parseInt(get("total_rooms", "totalrooms", "rooms")),
		OccupiedRooms:     parseInt(get("occupied_rooms", "occupiedrooms", "occupied")),
		OutOfOrderRooms:   parseInt(get("out_of_order_rooms", "ooo_rooms", "ooo")),
		RoomRevenueMinor:  parseMoneyMinor(get("room_revenue_minor", "room_revenue", "roomrev")),
		FBRevenueMinor:    parseMoneyMinor(get("fb_revenue_minor", "fb_revenue", "fbrev")),
		OtherRevenueMinor: parseMoneyMinor(get("other_revenue_minor", "other_revenue", "otherrev")),
		ArrivalsCount:     parseInt(get("arrivals", "arrivals_count")),
		DeparturesCount:   parseInt(get("departures", "departures_count")),
		InHouseGuests:     parseInt(get("in_house_guests", "in_house", "guests")),
		ReservationsCount: parseInt(get("reservations", "reservations_count")),
		CancelledCount:    parseInt(get("cancelled", "cancelled_count")),
		NoShowCount:       parseInt(get("no_show", "no_show_count")),
	}
	if _, hasConf := idx["confirmation_no"]; hasConf || idx["guest_name"] > 0 {
		for _, row := range rows[1:] {
			val := func(k string) string {
				if i, ok := idx[k]; ok && i < len(row) {
					return strings.TrimSpace(row[i])
				}
				return ""
			}
			in.Reservations = append(in.Reservations, hotel.ReservationRow{
				ConfirmationNo: firstNonEmpty(val("confirmation_no"), val("confirmation")),
				GuestName:      firstNonEmpty(val("guest_name"), val("guest")),
				RoomType:       val("room_type"),
				Status:         hotel.ReservationStatus(val("status")),
				ArrivalDate:    val("arrival_date"),
				DepartureDate:  val("departure_date"),
				Nights:         parseInt(val("nights")),
				RateMinor:      parseMoneyMinor(firstNonEmpty(val("rate_minor"), val("rate"))),
				Currency:       in.Currency,
				Channel:        val("channel"),
			})
		}
		if in.ReservationsCount == 0 {
			in.ReservationsCount = len(in.Reservations)
		}
	}
	return in, nil
}

func toConfigView(c hotel.OHIPConfig) OHIPConfigView {
	syncMode := string(c.SyncMode)
	if syncMode == "" {
		syncMode = string(hotel.SyncModeExport)
	}
	syncStatus := string(c.LastSyncStatus)
	if syncStatus == "" {
		syncStatus = string(hotel.SyncNever)
	}
	out := OHIPConfigView{
		Configured:     c.HostURL != "" && c.HotelID != "" && c.ClientID != "" && c.EncryptedClientSecret != "",
		Enabled:        c.Enabled,
		HostURL:        c.HostURL,
		HotelID:        c.HotelID,
		ClientID:       c.ClientID,
		SecretHint:     c.SecretHint,
		AppKeyHint:     c.AppKeyHint,
		SyncMode:       syncMode,
		LastSyncStatus: syncStatus,
		LastError:      c.LastError,
	}
	if c.LastSyncAt != nil {
		out.LastSyncAt = c.LastSyncAt.UTC().Format(time.RFC3339)
	}
	return out
}

func toSnapshotView(s hotel.PMSSnapshot) SnapshotView {
	readiness := s.Readiness
	if readiness == nil {
		readiness = []hotel.DepartmentReadiness{}
	}
	res := s.Reservations
	if res == nil {
		res = []hotel.ReservationRow{}
	}
	return SnapshotView{
		ID:                s.ID.String(),
		BusinessDate:      s.BusinessDate,
		Source:            string(s.Source),
		ExternalBatchRef:  s.ExternalBatchRef,
		Currency:          s.Currency,
		TotalRooms:        s.TotalRooms,
		AvailableRooms:    s.AvailableRooms(),
		OccupiedRooms:     s.OccupiedRooms,
		OutOfOrderRooms:   s.OutOfOrderRooms,
		OccupancyPct:      bpsToPct(s.OccupancyBasisPoints()),
		RoomRevenueMinor:  s.RoomRevenueMinor,
		FBRevenueMinor:    s.FBRevenueMinor,
		OtherRevenueMinor: s.OtherRevenueMinor,
		TotalRevenueMinor: s.TotalRevenueMinor(),
		ADRMinor:          s.ADRMinor(),
		RevPARMinor:       s.RevPARMinor(),
		ArrivalsCount:     s.ArrivalsCount,
		DeparturesCount:   s.DeparturesCount,
		InHouseGuests:     s.InHouseGuests,
		ReservationsCount: s.ReservationsCount,
		CancelledCount:    s.CancelledCount,
		NoShowCount:       s.NoShowCount,
		Readiness:         readiness,
		ReadinessPct:      s.ReadinessPct(),
		Reservations:      res,
		CapturedAt:        s.CapturedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Service) day(tz, raw string) (string, error) {
	if raw == "" {
		return finance.BusinessDate(s.clock.Now(), tz), nil
	}
	if err := hotel.ValidateBusinessDate(raw); err != nil {
		return "", err
	}
	return raw, nil
}

func previousDate(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}

func requireHotel(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindHotel {
		return errx.Invalid("hotel_required", "OHIP and hotel PMS belong to hotel workspaces")
	}
	return nil
}

func bpsToPct(bps int) float64 {
	return round1(float64(bps) / 100.0)
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func sanitizeSyncErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if r := []rune(msg); len(r) > 160 {
		return string(r[:160]) + "…"
	}
	return msg
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func parseInt(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return v
}

func parseMoneyMinor(raw string) int64 {
	s := strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if s == "" {
		return 0
	}
	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return int64(math.Round(f * 100))
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
