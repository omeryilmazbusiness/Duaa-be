package hotel

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type SyncMode string
type SyncStatus string
type PMSSource string
type Freshness string
type ReservationStatus string

const (
	SyncModeOHIP   SyncMode = "ohip_api"
	SyncModeExport SyncMode = "opera_export"
)

const (
	SyncNever SyncStatus = "never"
	SyncOK    SyncStatus = "ok"
	SyncError SyncStatus = "error"
)

const (
	SourceOHIPLive    PMSSource = "ohip_live"
	SourceOPERAExport PMSSource = "opera_export"
	SourceManual      PMSSource = "manual"
)

const (
	FreshCurrent     Freshness = "current"
	FreshDelayed     Freshness = "delayed"
	FreshStale       Freshness = "stale"
	FreshUnavailable Freshness = "unavailable"
)

const (
	ResReserved   ReservationStatus = "reserved"
	ResCheckedIn  ReservationStatus = "checked_in"
	ResCheckedOut ReservationStatus = "checked_out"
	ResCancelled  ReservationStatus = "cancelled"
	ResNoShow     ReservationStatus = "no_show"
)

const (
	MaxHotelRooms       = 10_000
	MaxHotelReservations = 200
	MaxHotelReadiness   = 24
	DelayedAfter        = 90 * time.Minute
	StaleAfter          = 6 * time.Hour
)

// OHIPConfig stores per-hotel Oracle Hospitality Integration Platform settings.
// ClientSecret and AppKey are encrypted at rest; only hints leave the store.
type OHIPConfig struct {
	WorkspaceID           id.ID
	Enabled               bool
	HostURL               string
	HotelID               string
	ClientID              string
	EncryptedClientSecret string
	SecretHint            string
	EncryptedAppKey       string
	AppKeyHint            string
	SyncMode              SyncMode
	LastSyncAt            *time.Time
	LastSyncStatus        SyncStatus
	LastError             string
	UpdatedBy             id.ID
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type DepartmentReadiness struct {
	Department string `json:"department"`
	Total      int    `json:"total"`
	Completed  int    `json:"completed"`
	Note       string `json:"note,omitempty"`
}

type ReservationRow struct {
	ConfirmationNo string            `json:"confirmationNo"`
	GuestName      string            `json:"guestName"`
	RoomType       string            `json:"roomType"`
	Status         ReservationStatus `json:"status"`
	ArrivalDate    string            `json:"arrivalDate"`
	DepartureDate  string            `json:"departureDate"`
	Nights         int               `json:"nights"`
	RateMinor      int64             `json:"rateMinor"`
	Currency       string            `json:"currency"`
	Channel        string            `json:"channel,omitempty"`
	VIP            bool              `json:"vip,omitempty"`
}

// PMSSnapshot is one canonical daily hotel state from OHIP, an OPERA export or manual entry.
// Money uses integer minor units (e.g. USD cents or IQD whole/fils) — never binary float.
type PMSSnapshot struct {
	ID                id.ID
	WorkspaceID       id.ID
	BusinessDate      string
	Source            PMSSource
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
	Readiness         []DepartmentReadiness
	Reservations      []ReservationRow
	CapturedAt        time.Time
	RecordedBy        id.ID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func ParseSyncMode(raw string) (SyncMode, error) {
	switch SyncMode(strings.ToLower(strings.TrimSpace(raw))) {
	case "", SyncModeExport:
		return SyncModeExport, nil
	case SyncModeOHIP:
		return SyncModeOHIP, nil
	default:
		return "", errx.Invalid("invalid_sync_mode", "syncMode must be ohip_api or opera_export")
	}
}

func ParsePMSSource(raw string) (PMSSource, error) {
	switch PMSSource(strings.ToLower(strings.TrimSpace(raw))) {
	case SourceOHIPLive, SourceOPERAExport, SourceManual:
		return PMSSource(strings.ToLower(strings.TrimSpace(raw))), nil
	case "":
		return SourceManual, nil
	default:
		return "", errx.Invalid("invalid_pms_source", "source must be ohip_live, opera_export or manual")
	}
}

func NormalizeHostURL(raw string) (string, error) {
	s := strings.TrimRight(strings.TrimSpace(raw), "/")
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", errx.Invalid("invalid_ohip_host", "OHIP host URL must be a valid URL")
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	local := host == "127.0.0.1" || host == "localhost"
	if scheme != "https" && !(scheme == "http" && local) {
		return "", errx.Invalid("invalid_ohip_host", "OHIP host URL must use https")
	}
	return u.Scheme + "://" + u.Host + strings.TrimRight(u.Path, "/"), nil
}

func NormalizeCurrency(raw string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return "USD", nil
	}
	if len(s) != 3 {
		return "", errx.Invalid("invalid_currency", "currency must be a 3-letter ISO code")
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return "", errx.Invalid("invalid_currency", "currency must be a 3-letter ISO code")
		}
	}
	return s, nil
}

func ValidateBusinessDate(raw string) error {
	if len(raw) != 10 {
		return errx.Invalid("invalid_business_date", "businessDate must be YYYY-MM-DD")
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return errx.Invalid("invalid_business_date", "businessDate must be YYYY-MM-DD")
	}
	return nil
}

func (s *PMSSnapshot) NormalizeAndValidate() error {
	if s.WorkspaceID == id.Zero() || s.RecordedBy == id.Zero() {
		return errx.Invalid("invalid_pms_refs", "workspace and recorder are required")
	}
	if err := ValidateBusinessDate(s.BusinessDate); err != nil {
		return err
	}
	src, err := ParsePMSSource(string(s.Source))
	if err != nil {
		return err
	}
	s.Source = src
	cur, err := NormalizeCurrency(s.Currency)
	if err != nil {
		return err
	}
	s.Currency = cur
	s.ExternalBatchRef = strings.TrimSpace(s.ExternalBatchRef)
	if utf8.RuneCountInString(s.ExternalBatchRef) > 120 {
		return errx.Invalid("invalid_batch_ref", "externalBatchRef is too long")
	}
	if s.TotalRooms < 0 || s.TotalRooms > MaxHotelRooms || s.OutOfOrderRooms < 0 || s.OutOfOrderRooms > s.TotalRooms {
		return errx.Invalid("invalid_room_counts", "room counts are invalid")
	}
	avail := s.AvailableRooms()
	if s.OccupiedRooms < 0 || (avail > 0 && s.OccupiedRooms > avail) || (avail == 0 && s.OccupiedRooms > s.TotalRooms) {
		return errx.Invalid("invalid_occupied_rooms", "occupied rooms exceed available rooms")
	}
	if s.RoomRevenueMinor < 0 || s.FBRevenueMinor < 0 || s.OtherRevenueMinor < 0 {
		return errx.Invalid("invalid_revenue", "revenue cannot be negative")
	}
	if s.ArrivalsCount < 0 || s.DeparturesCount < 0 || s.InHouseGuests < 0 || s.ReservationsCount < 0 || s.CancelledCount < 0 || s.NoShowCount < 0 {
		return errx.Invalid("invalid_counts", "operational counts cannot be negative")
	}
	if len(s.Readiness) > MaxHotelReadiness {
		s.Readiness = s.Readiness[:MaxHotelReadiness]
	}
	for i := range s.Readiness {
		s.Readiness[i].Department = strings.TrimSpace(s.Readiness[i].Department)
		s.Readiness[i].Note = strings.TrimSpace(s.Readiness[i].Note)
		if s.Readiness[i].Total < 0 {
			s.Readiness[i].Total = 0
		}
		if s.Readiness[i].Completed < 0 {
			s.Readiness[i].Completed = 0
		}
		if s.Readiness[i].Completed > s.Readiness[i].Total {
			s.Readiness[i].Completed = s.Readiness[i].Total
		}
	}
	if len(s.Reservations) > MaxHotelReservations {
		s.Reservations = s.Reservations[:MaxHotelReservations]
	}
	for i := range s.Reservations {
		s.Reservations[i].ConfirmationNo = strings.TrimSpace(s.Reservations[i].ConfirmationNo)
		s.Reservations[i].GuestName = strings.TrimSpace(s.Reservations[i].GuestName)
		s.Reservations[i].RoomType = strings.TrimSpace(s.Reservations[i].RoomType)
		s.Reservations[i].Channel = strings.TrimSpace(s.Reservations[i].Channel)
		if s.Reservations[i].Currency == "" {
			s.Reservations[i].Currency = s.Currency
		}
		s.Reservations[i].Status = normalizeResStatus(string(s.Reservations[i].Status))
	}
	if s.ReservationsCount == 0 && len(s.Reservations) > 0 {
		s.ReservationsCount = len(s.Reservations)
	}
	if s.ID == id.Zero() {
		s.ID = id.New()
	}
	return nil
}

func normalizeResStatus(raw string) ReservationStatus {
	switch ReservationStatus(strings.ToLower(strings.TrimSpace(raw))) {
	case ResCheckedIn, "inhouse", "in_house":
		return ResCheckedIn
	case ResCheckedOut, "checkedout", "departed":
		return ResCheckedOut
	case ResCancelled, "canceled":
		return ResCancelled
	case ResNoShow, "noshow":
		return ResNoShow
	default:
		return ResReserved
	}
}

func (s PMSSnapshot) AvailableRooms() int {
	avail := s.TotalRooms - s.OutOfOrderRooms
	if avail < 0 {
		return 0
	}
	return avail
}

// OccupancyBasisPoints returns 0–10000 (e.g. 8250 = 82.50%).
func (s PMSSnapshot) OccupancyBasisPoints() int {
	avail := s.AvailableRooms()
	if avail <= 0 {
		return 0
	}
	bps := (s.OccupiedRooms * 10_000) / avail
	if bps < 0 {
		return 0
	}
	if bps > 10_000 {
		return 10_000
	}
	return bps
}

func (s PMSSnapshot) TotalRevenueMinor() int64 {
	return s.RoomRevenueMinor + s.FBRevenueMinor + s.OtherRevenueMinor
}

func (s PMSSnapshot) ADRMinor() int64 {
	if s.OccupiedRooms <= 0 {
		return 0
	}
	return s.RoomRevenueMinor / int64(s.OccupiedRooms)
}

func (s PMSSnapshot) RevPARMinor() int64 {
	avail := s.AvailableRooms()
	if avail <= 0 {
		return 0
	}
	return s.RoomRevenueMinor / int64(avail)
}

func (s PMSSnapshot) ReadinessPct() int {
	total, done := 0, 0
	for _, r := range s.Readiness {
		total += r.Total
		done += r.Completed
	}
	if total <= 0 {
		return 0
	}
	return (done * 100) / total
}

// ComputeFreshness follows PDF CON-03 / AT-10: failed connector keeps last valid values as stale, never zero.
func ComputeFreshness(now time.Time, snap *PMSSnapshot, lastStatus SyncStatus) Freshness {
	if snap == nil {
		return FreshUnavailable
	}
	if lastStatus == SyncError {
		return FreshStale
	}
	age := now.Sub(snap.CapturedAt)
	if age > StaleAfter {
		return FreshStale
	}
	if age > DelayedAfter {
		return FreshDelayed
	}
	return FreshCurrent
}

func KeyHint(secret string) string {
	s := strings.TrimSpace(secret)
	if len(s) <= 4 {
		return "••••"
	}
	return "••••" + s[len(s)-4:]
}

func OHIPConfigNotFoundError() error {
	return errx.NotFound("ohip_config_not_found", "OHIP configuration not found")
}

func PMSSnapshotNotFoundError() error {
	return errx.NotFound("pms_snapshot_not_found", "PMS snapshot not found")
}
