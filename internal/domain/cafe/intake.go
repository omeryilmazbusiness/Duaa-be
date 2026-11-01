package cafe

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	MaxIntakeNote = 500
	MinIntakeNote = 2
)

// Intake is the manager's daily close note: shift, waste, incident.
// It never invents a fire cost, a POS total, or a shift that is not recorded.
type Intake struct {
	ID           id.ID
	WorkspaceID  id.ID
	BusinessDate string
	ShiftName    string
	CloseNote    string
	WasteNote    string
	IncidentNote string
	AuthorUserID id.ID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func ParseBusinessDate(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errx.Invalid("invalid_date", "business date is required")
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", errx.Invalid("invalid_date", "business date must be YYYY-MM-DD")
	}
	return s, nil
}

func RecordedShiftNames(shifts []Shift) []string {
	out := make([]string, 0, len(shifts))
	for _, s := range shifts {
		if n := strings.TrimSpace(s.Name); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func (i *Intake) Normalize() {
	if i == nil {
		return
	}
	i.BusinessDate = strings.TrimSpace(i.BusinessDate)
	i.ShiftName = clip(strings.TrimSpace(i.ShiftName), MaxShiftNameLen)
	i.CloseNote = clip(strings.TrimSpace(i.CloseNote), MaxIntakeNote)
	i.WasteNote = clip(strings.TrimSpace(i.WasteNote), MaxIntakeNote)
	i.IncidentNote = clip(strings.TrimSpace(i.IncidentNote), MaxIntakeNote)
}

func (i Intake) HasBody() bool {
	return utf8.RuneCountInString(i.CloseNote) >= MinIntakeNote ||
		utf8.RuneCountInString(i.WasteNote) >= MinIntakeNote ||
		utf8.RuneCountInString(i.IncidentNote) >= MinIntakeNote
}

func (i Intake) Validate(recordedShifts []string) error {
	if i.WorkspaceID == id.Zero() || i.AuthorUserID == id.Zero() {
		return errx.Invalid("invalid_intake", "workspace and author are required")
	}
	if _, err := ParseBusinessDate(i.BusinessDate); err != nil {
		return err
	}
	if i.ShiftName != "" {
		if len(recordedShifts) == 0 {
			return errx.Invalid("shift_not_recorded", "no shift is recorded. Do not invent one")
		}
		if !shiftKnown(recordedShifts, i.ShiftName) {
			return errx.Invalid("unknown_shift", "shift is not on the cafe definition")
		}
	}
	if !i.HasBody() {
		return errx.Invalid("empty_intake", "write the close, waste or an incident")
	}
	if utf8.RuneCountInString(i.CloseNote) > MaxIntakeNote ||
		utf8.RuneCountInString(i.WasteNote) > MaxIntakeNote ||
		utf8.RuneCountInString(i.IncidentNote) > MaxIntakeNote {
		return errx.Invalid("invalid_intake_note", "note is too long")
	}
	return nil
}

func UpsertIntake(cur *Intake, workspaceID, author id.ID, date, shift, closeNote, wasteNote, incidentNote string, recordedShifts []string, now time.Time) (Intake, error) {
	rec := Intake{
		WorkspaceID:  workspaceID,
		AuthorUserID: author,
		BusinessDate: date,
		ShiftName:    shift,
		CloseNote:    closeNote,
		WasteNote:    wasteNote,
		IncidentNote: incidentNote,
		UpdatedAt:    now,
	}
	if cur != nil && cur.ID != id.Zero() {
		rec.ID = cur.ID
		rec.CreatedAt = cur.CreatedAt
	} else {
		rec.ID = id.New()
		rec.CreatedAt = now
	}
	rec.Normalize()
	if err := rec.Validate(recordedShifts); err != nil {
		return Intake{}, err
	}
	return rec, nil
}

func shiftKnown(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}
