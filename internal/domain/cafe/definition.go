package cafe

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// POS is declared, not live. "connected" is rejected until a POS adapter exists.
type POSStatus string

const (
	POSUnknown POSStatus = "unknown"
	POSNone    POSStatus = "none"
	POSNamed   POSStatus = "named"
)

type Area string

const (
	AreaIndoor  Area = "indoor"
	AreaGarden  Area = "garden"
	AreaCounter Area = "counter"
	AreaTable   Area = "table"
	AreaOther   Area = "other"
)

const (
	MaxNameLen       = 80
	MaxAddressLen    = 200
	MaxVendorLen     = 80
	MaxShiftNameLen  = 40
	MaxAfterHoursLen = 280
	MaxBranches      = 32
	MaxQRSites       = 128
	MaxShifts        = 24
	MaxSLAMinutes    = 24 * 60
	MaxCompensation  = 1_000_000_000
	missingName      = "name"
	missingTimezone  = "timezone"
	missingCurrency  = "currency"
	missingBranch    = "branch"
	missingQRSite    = "qr_site"
	missingSLA       = "sla"
)

var (
	currencyPat = regexp.MustCompile(`^[A-Z]{3}$`)
	clockPat    = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

type Branch struct {
	ID      id.ID
	Name    string
	Address string
	Active  bool
}

type QRSite struct {
	ID       id.ID
	BranchID id.ID
	Label    string
	Area     Area
	Active   bool
}

type Shift struct {
	ID    id.ID
	Name  string
	Start string
	End   string
}

type SLA struct {
	AckMinutes           int
	ResolveMinutes       int
	AfterHoursMessage    string
	CompensationLimit    int64
	CompensationCurrency string
}

// Definition is the cafe identity aggregate: one document per workspace.
// Empty collections are valid. ReadyForReviews is derived, never stored.
type Definition struct {
	WorkspaceID id.ID
	DisplayName string
	Currency    string
	Timezone    string
	POSVendor   string
	POSStatus   POSStatus
	Branches    []Branch
	QRSites     []QRSite
	Shifts      []Shift
	SLA         SLA
	UpdatedAt   time.Time
	UpdatedBy   id.ID
}

type Completeness struct {
	ReadyForReviews bool     `json:"readyForReviews"`
	HasName         bool     `json:"hasName"`
	HasTimezone     bool     `json:"hasTimezone"`
	HasCurrency     bool     `json:"hasCurrency"`
	HasBranch       bool     `json:"hasBranch"`
	HasQRSite       bool     `json:"hasQRSite"`
	HasSLA          bool     `json:"hasSLA"`
	Missing         []string `json:"missing"`
}

func Seed(workspaceID id.ID, name, timezone string) Definition {
	return Definition{
		WorkspaceID: workspaceID,
		DisplayName: strings.TrimSpace(name),
		Timezone:    strings.TrimSpace(timezone),
		POSStatus:   POSUnknown,
		Branches:    []Branch{},
		QRSites:     []QRSite{},
		Shifts:      []Shift{},
	}
}

func (d Definition) Clone() Definition {
	out := d
	out.Branches = append([]Branch(nil), d.Branches...)
	out.QRSites = append([]QRSite(nil), d.QRSites...)
	out.Shifts = append([]Shift(nil), d.Shifts...)
	out.SLA = d.SLA
	return out
}

func (d *Definition) Normalize() {
	if d == nil {
		return
	}
	d.DisplayName = clip(strings.TrimSpace(d.DisplayName), MaxNameLen)
	d.Currency = strings.ToUpper(strings.TrimSpace(d.Currency))
	d.Timezone = strings.TrimSpace(d.Timezone)
	d.POSVendor = clip(strings.TrimSpace(d.POSVendor), MaxVendorLen)
	d.POSStatus = POSStatus(strings.ToLower(strings.TrimSpace(string(d.POSStatus))))
	if d.POSStatus == "" {
		d.POSStatus = POSUnknown
	}
	if d.POSVendor != "" && d.POSStatus == POSUnknown {
		d.POSStatus = POSNamed
	}
	if d.POSStatus == POSNone {
		d.POSVendor = ""
	}
	if d.Branches == nil {
		d.Branches = []Branch{}
	}
	if d.QRSites == nil {
		d.QRSites = []QRSite{}
	}
	if d.Shifts == nil {
		d.Shifts = []Shift{}
	}
	for i := range d.Branches {
		if d.Branches[i].ID == id.Zero() {
			d.Branches[i].ID = id.New()
		}
		d.Branches[i].Name = clip(strings.TrimSpace(d.Branches[i].Name), MaxNameLen)
		d.Branches[i].Address = clip(strings.TrimSpace(d.Branches[i].Address), MaxAddressLen)
	}
	for i := range d.QRSites {
		if d.QRSites[i].ID == id.Zero() {
			d.QRSites[i].ID = id.New()
		}
		d.QRSites[i].Label = clip(strings.TrimSpace(d.QRSites[i].Label), MaxNameLen)
		d.QRSites[i].Area = Area(strings.ToLower(strings.TrimSpace(string(d.QRSites[i].Area))))
		if d.QRSites[i].Area == "" {
			d.QRSites[i].Area = AreaOther
		}
	}
	for i := range d.Shifts {
		if d.Shifts[i].ID == id.Zero() {
			d.Shifts[i].ID = id.New()
		}
		d.Shifts[i].Name = clip(strings.TrimSpace(d.Shifts[i].Name), MaxShiftNameLen)
		d.Shifts[i].Start = strings.TrimSpace(d.Shifts[i].Start)
		d.Shifts[i].End = strings.TrimSpace(d.Shifts[i].End)
	}
	d.SLA.AfterHoursMessage = clip(strings.TrimSpace(d.SLA.AfterHoursMessage), MaxAfterHoursLen)
	d.SLA.CompensationCurrency = strings.ToUpper(strings.TrimSpace(d.SLA.CompensationCurrency))
	if len(d.Branches) == 1 {
		only := d.Branches[0].ID
		for i := range d.QRSites {
			if d.QRSites[i].BranchID == id.Zero() {
				d.QRSites[i].BranchID = only
			}
		}
	}
}

func (d Definition) Validate() error {
	if d.WorkspaceID == id.Zero() {
		return errx.Invalid("invalid_workspace", "workspace is required")
	}
	if utf8.RuneCountInString(d.DisplayName) > MaxNameLen {
		return errx.Invalid("invalid_name", "official name is too long")
	}
	if d.Currency != "" && !currencyPat.MatchString(d.Currency) {
		return errx.Invalid("invalid_currency", "currency must be a 3-letter ISO 4217 code")
	}
	if d.Timezone != "" {
		if _, err := time.LoadLocation(d.Timezone); err != nil {
			return errx.Invalid("invalid_timezone", "timezone must be a valid IANA name")
		}
	}
	if err := d.POSStatus.validate(d.POSVendor); err != nil {
		return err
	}
	if len(d.Branches) > MaxBranches {
		return errx.Invalid("too_many_branches", "too many branches")
	}
	if len(d.QRSites) > MaxQRSites {
		return errx.Invalid("too_many_qr_sites", "too many QR locations")
	}
	if len(d.Shifts) > MaxShifts {
		return errx.Invalid("too_many_shifts", "too many shifts")
	}
	seenBranch := make(map[id.ID]struct{}, len(d.Branches))
	activeBranch := make(map[id.ID]struct{}, len(d.Branches))
	for _, b := range d.Branches {
		if b.ID == id.Zero() {
			return errx.Invalid("invalid_branch", "branch id is required")
		}
		if _, dup := seenBranch[b.ID]; dup {
			return errx.Invalid("duplicate_branch", "branch ids must be unique")
		}
		seenBranch[b.ID] = struct{}{}
		if strings.TrimSpace(b.Name) == "" {
			return errx.Invalid("invalid_branch_name", "branch name is required")
		}
		if b.Active {
			activeBranch[b.ID] = struct{}{}
		}
	}
	seenQR := make(map[id.ID]struct{}, len(d.QRSites))
	seenTable := make(map[string]struct{}, len(d.QRSites))
	for _, q := range d.QRSites {
		if q.ID == id.Zero() {
			return errx.Invalid("invalid_qr_site", "QR location id is required")
		}
		if _, dup := seenQR[q.ID]; dup {
			return errx.Invalid("duplicate_qr_site", "QR location ids must be unique")
		}
		seenQR[q.ID] = struct{}{}
		if strings.TrimSpace(q.Label) == "" {
			return errx.Invalid("invalid_qr_label", "QR location label is required")
		}
		if !q.Area.valid() {
			return errx.Invalid("invalid_qr_area", "QR area must be indoor, garden, counter, table or other")
		}
		if q.BranchID != id.Zero() {
			if _, ok := seenBranch[q.BranchID]; !ok {
				return errx.Invalid("invalid_qr_branch", "QR location must belong to a known branch")
			}
		} else if len(d.Branches) > 1 {
			return errx.Invalid("invalid_qr_branch", "QR location must name its branch")
		}
		if q.Area == AreaTable {
			key := strings.ToLower(q.Label)
			if _, dup := seenTable[key]; dup {
				return errx.Invalid("duplicate_table", "each table number is used once")
			}
			seenTable[key] = struct{}{}
		}
	}
	seenShift := make(map[id.ID]struct{}, len(d.Shifts))
	for _, s := range d.Shifts {
		if s.ID == id.Zero() {
			return errx.Invalid("invalid_shift", "shift id is required")
		}
		if _, dup := seenShift[s.ID]; dup {
			return errx.Invalid("duplicate_shift", "shift ids must be unique")
		}
		seenShift[s.ID] = struct{}{}
		if strings.TrimSpace(s.Name) == "" {
			return errx.Invalid("invalid_shift_name", "shift name is required")
		}
		if !clockPat.MatchString(s.Start) || !clockPat.MatchString(s.End) {
			return errx.Invalid("invalid_shift_time", "shift times must be HH:MM")
		}
	}
	return d.SLA.validate()
}

func (d Definition) Completeness() Completeness {
	c := Completeness{
		HasName:     strings.TrimSpace(d.DisplayName) != "",
		HasTimezone: d.Timezone != "" && timezoneOK(d.Timezone),
		HasCurrency: currencyPat.MatchString(d.Currency),
		HasSLA:      d.SLA.AckMinutes > 0,
		Missing:     make([]string, 0, 6),
	}
	for _, b := range d.Branches {
		if b.Active {
			c.HasBranch = true
			break
		}
	}
	for _, q := range d.QRSites {
		if q.Active {
			c.HasQRSite = true
			break
		}
	}
	addMissing(&c, c.HasName, missingName)
	addMissing(&c, c.HasTimezone, missingTimezone)
	addMissing(&c, c.HasCurrency, missingCurrency)
	addMissing(&c, c.HasBranch, missingBranch)
	addMissing(&c, c.HasQRSite, missingQRSite)
	addMissing(&c, c.HasSLA, missingSLA)
	c.ReadyForReviews = len(c.Missing) == 0
	return c
}

func (s POSStatus) validate(vendor string) error {
	switch s {
	case POSUnknown:
		if vendor != "" {
			return errx.Invalid("invalid_pos", "name the POS or leave the vendor empty")
		}
		return nil
	case POSNone:
		if vendor != "" {
			return errx.Invalid("invalid_pos", "a cafe without POS cannot name a vendor")
		}
		return nil
	case POSNamed:
		if vendor == "" {
			return errx.Invalid("invalid_pos_vendor", "POS vendor is required when a system is named")
		}
		return nil
	case "connected":
		return errx.Invalid("pos_not_connected", "live POS is not wired; mark it named until an adapter exists")
	default:
		return errx.Invalid("invalid_pos_status", "POS status must be unknown, none or named")
	}
}

func (a Area) valid() bool {
	switch a {
	case AreaIndoor, AreaGarden, AreaCounter, AreaTable, AreaOther:
		return true
	default:
		return false
	}
}

func (s SLA) validate() error {
	if s.AckMinutes < 0 || s.AckMinutes > MaxSLAMinutes {
		return errx.Invalid("invalid_sla_ack", "acknowledgement minutes must be between 0 and 1440")
	}
	if s.ResolveMinutes < 0 || s.ResolveMinutes > MaxSLAMinutes {
		return errx.Invalid("invalid_sla_resolve", "resolve minutes must be between 0 and 1440")
	}
	if s.AckMinutes > 0 && s.ResolveMinutes > 0 && s.ResolveMinutes < s.AckMinutes {
		return errx.Invalid("invalid_sla_order", "resolve time cannot be shorter than acknowledgement")
	}
	if s.CompensationLimit < 0 || s.CompensationLimit > MaxCompensation {
		return errx.Invalid("invalid_compensation", "compensation limit is out of range")
	}
	if s.CompensationLimit > 0 && !currencyPat.MatchString(s.CompensationCurrency) {
		return errx.Invalid("invalid_compensation_currency", "compensation currency is required when a limit is set")
	}
	if s.CompensationCurrency != "" && !currencyPat.MatchString(s.CompensationCurrency) {
		return errx.Invalid("invalid_compensation_currency", "compensation currency must be a 3-letter ISO 4217 code")
	}
	return nil
}

func timezoneOK(name string) bool {
	_, err := time.LoadLocation(name)
	return err == nil
}

func addMissing(c *Completeness, ok bool, key string) {
	if !ok {
		c.Missing = append(c.Missing, key)
	}
}

func clip(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}
