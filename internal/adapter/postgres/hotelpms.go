package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) GetOHIPConfig(ctx context.Context, workspaceID id.ID) (hotel.OHIPConfig, error) {
	var c hotel.OHIPConfig
	var mode, status string
	err := s.pool.QueryRow(ctx, `SELECT workspace_id, enabled, host_url, hotel_id, client_id,
		encrypted_client_secret, secret_hint, encrypted_app_key, app_key_hint, sync_mode,
		last_sync_at, last_sync_status, last_error, updated_by, created_at, updated_at
		FROM hotel_ohip_configs WHERE workspace_id=$1`, workspaceID).
		Scan(&c.WorkspaceID, &c.Enabled, &c.HostURL, &c.HotelID, &c.ClientID,
			&c.EncryptedClientSecret, &c.SecretHint, &c.EncryptedAppKey, &c.AppKeyHint, &mode,
			&c.LastSyncAt, &status, &c.LastError, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return hotel.OHIPConfig{}, hotel.OHIPConfigNotFoundError()
	}
	if err != nil {
		return hotel.OHIPConfig{}, err
	}
	c.SyncMode = hotel.SyncMode(mode)
	c.LastSyncStatus = hotel.SyncStatus(status)
	return c, nil
}

func (s *Store) SaveOHIPConfig(ctx context.Context, c hotel.OHIPConfig) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO hotel_ohip_configs (
		workspace_id, enabled, host_url, hotel_id, client_id,
		encrypted_client_secret, secret_hint, encrypted_app_key, app_key_hint, sync_mode,
		last_sync_at, last_sync_status, last_error, updated_by, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	ON CONFLICT (workspace_id) DO UPDATE SET
		enabled=EXCLUDED.enabled, host_url=EXCLUDED.host_url, hotel_id=EXCLUDED.hotel_id, client_id=EXCLUDED.client_id,
		encrypted_client_secret=EXCLUDED.encrypted_client_secret, secret_hint=EXCLUDED.secret_hint,
		encrypted_app_key=EXCLUDED.encrypted_app_key, app_key_hint=EXCLUDED.app_key_hint,
		sync_mode=EXCLUDED.sync_mode, last_sync_at=EXCLUDED.last_sync_at, last_sync_status=EXCLUDED.last_sync_status,
		last_error=EXCLUDED.last_error, updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
		c.WorkspaceID, c.Enabled, c.HostURL, c.HotelID, c.ClientID,
		c.EncryptedClientSecret, c.SecretHint, c.EncryptedAppKey, c.AppKeyHint, string(c.SyncMode),
		c.LastSyncAt, string(c.LastSyncStatus), c.LastError, c.UpdatedBy, c.CreatedAt, c.UpdatedAt)
	return err
}

func (s *Store) GetHotelProfile(ctx context.Context, workspaceID id.ID) (hotel.Profile, error) {
	var p hotel.Profile
	err := s.pool.QueryRow(ctx, `SELECT workspace_id, display_name, total_rooms, currency, timezone,
		star_rating, city, address, phone, gm_name, notes, updated_by, updated_at
		FROM hotel_profiles WHERE workspace_id=$1`, workspaceID).
		Scan(&p.WorkspaceID, &p.DisplayName, &p.TotalRooms, &p.Currency, &p.Timezone,
			&p.StarRating, &p.City, &p.Address, &p.Phone, &p.GMName, &p.Notes, &p.UpdatedBy, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return hotel.Profile{}, hotel.ProfileNotFoundError()
	}
	return p, err
}

func (s *Store) SaveHotelProfile(ctx context.Context, p hotel.Profile) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO hotel_profiles (
		workspace_id, display_name, total_rooms, currency, timezone,
		star_rating, city, address, phone, gm_name, notes, updated_by, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	ON CONFLICT (workspace_id) DO UPDATE SET
		display_name=EXCLUDED.display_name, total_rooms=EXCLUDED.total_rooms, currency=EXCLUDED.currency,
		timezone=EXCLUDED.timezone, star_rating=EXCLUDED.star_rating, city=EXCLUDED.city,
		address=EXCLUDED.address, phone=EXCLUDED.phone, gm_name=EXCLUDED.gm_name, notes=EXCLUDED.notes,
		updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
		p.WorkspaceID, p.DisplayName, p.TotalRooms, p.Currency, p.Timezone,
		p.StarRating, p.City, p.Address, p.Phone, p.GMName, p.Notes, p.UpdatedBy, p.UpdatedAt)
	return err
}

const pmsSelect = `SELECT id, workspace_id, business_date::text, source, external_batch_ref, currency,
	total_rooms, occupied_rooms, out_of_order_rooms, room_revenue_minor, fb_revenue_minor, other_revenue_minor,
	arrivals_count, departures_count, in_house_guests, reservations_count, cancelled_count, no_show_count,
	readiness, reservations, captured_at, recorded_by, created_at, updated_at FROM hotel_pms_snapshots`

func (s *Store) GetPMSSnapshot(ctx context.Context, workspaceID id.ID, businessDate string) (hotel.PMSSnapshot, error) {
	row := s.pool.QueryRow(ctx, pmsSelect+` WHERE workspace_id=$1 AND business_date=$2`, workspaceID, businessDate)
	snap, err := scanPMSSnapshot(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return hotel.PMSSnapshot{}, hotel.PMSSnapshotNotFoundError()
	}
	return snap, err
}

func (s *Store) SavePMSSnapshot(ctx context.Context, snap hotel.PMSSnapshot) error {
	readyRaw, err := json.Marshal(snap.Readiness)
	if err != nil {
		return err
	}
	resRaw, err := json.Marshal(snap.Reservations)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO hotel_pms_snapshots (
		id, workspace_id, business_date, source, external_batch_ref, currency,
		total_rooms, occupied_rooms, out_of_order_rooms, room_revenue_minor, fb_revenue_minor, other_revenue_minor,
		arrivals_count, departures_count, in_house_guests, reservations_count, cancelled_count, no_show_count,
		readiness, reservations, captured_at, recorded_by, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)
	ON CONFLICT (workspace_id, business_date) DO UPDATE SET
		source=EXCLUDED.source, external_batch_ref=EXCLUDED.external_batch_ref, currency=EXCLUDED.currency,
		total_rooms=EXCLUDED.total_rooms, occupied_rooms=EXCLUDED.occupied_rooms, out_of_order_rooms=EXCLUDED.out_of_order_rooms,
		room_revenue_minor=EXCLUDED.room_revenue_minor, fb_revenue_minor=EXCLUDED.fb_revenue_minor, other_revenue_minor=EXCLUDED.other_revenue_minor,
		arrivals_count=EXCLUDED.arrivals_count, departures_count=EXCLUDED.departures_count, in_house_guests=EXCLUDED.in_house_guests,
		reservations_count=EXCLUDED.reservations_count, cancelled_count=EXCLUDED.cancelled_count, no_show_count=EXCLUDED.no_show_count,
		readiness=EXCLUDED.readiness, reservations=EXCLUDED.reservations, captured_at=EXCLUDED.captured_at,
		recorded_by=EXCLUDED.recorded_by, updated_at=EXCLUDED.updated_at`,
		snap.ID, snap.WorkspaceID, snap.BusinessDate, string(snap.Source), snap.ExternalBatchRef, snap.Currency,
		snap.TotalRooms, snap.OccupiedRooms, snap.OutOfOrderRooms, snap.RoomRevenueMinor, snap.FBRevenueMinor, snap.OtherRevenueMinor,
		snap.ArrivalsCount, snap.DeparturesCount, snap.InHouseGuests, snap.ReservationsCount, snap.CancelledCount, snap.NoShowCount,
		readyRaw, resRaw, snap.CapturedAt, snap.RecordedBy, snap.CreatedAt, snap.UpdatedAt)
	return err
}

func (s *Store) ListPMSSnapshots(ctx context.Context, workspaceID id.ID, limit int) ([]hotel.PMSSnapshot, error) {
	if limit <= 0 {
		limit = 7
	}
	rows, err := s.pool.Query(ctx, pmsSelect+` WHERE workspace_id=$1 ORDER BY business_date DESC LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]hotel.PMSSnapshot, 0)
	for rows.Next() {
		snap, err := scanPMSSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

func scanPMSSnapshot(row pgx.Row) (hotel.PMSSnapshot, error) {
	var snap hotel.PMSSnapshot
	var src string
	var readyRaw, resRaw []byte
	err := row.Scan(
		&snap.ID, &snap.WorkspaceID, &snap.BusinessDate, &src, &snap.ExternalBatchRef, &snap.Currency,
		&snap.TotalRooms, &snap.OccupiedRooms, &snap.OutOfOrderRooms, &snap.RoomRevenueMinor, &snap.FBRevenueMinor, &snap.OtherRevenueMinor,
		&snap.ArrivalsCount, &snap.DeparturesCount, &snap.InHouseGuests, &snap.ReservationsCount, &snap.CancelledCount, &snap.NoShowCount,
		&readyRaw, &resRaw, &snap.CapturedAt, &snap.RecordedBy, &snap.CreatedAt, &snap.UpdatedAt,
	)
	if err != nil {
		return hotel.PMSSnapshot{}, err
	}
	snap.Source = hotel.PMSSource(src)
	_ = json.Unmarshal(readyRaw, &snap.Readiness)
	_ = json.Unmarshal(resRaw, &snap.Reservations)
	return snap, nil
}
