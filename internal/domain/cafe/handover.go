package cafe

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	HandoverOpen     = "open"
	HandoverAccepted = "accepted"
	MaxHandoverNote  = 500
	MaxHandoverTitle = 80
)

// HandoverRef is one live item frozen at handoff. Title is the recorded label, not invented work.
type HandoverRef struct {
	ID    id.ID  `json:"id"`
	Title string `json:"title"`
}

// Handover is the outgoing manager's snapshot: open work, recoveries, breakdowns.
// The incoming manager accepts it. Empty lists are valid.
type Handover struct {
	ID           id.ID
	WorkspaceID  id.ID
	BusinessDate string
	FromShift    string
	ToShift      string
	Status       string
	Note         string
	Tasks        []HandoverRef
	Recoveries   []HandoverRef
	Breakdowns   []HandoverRef
	OpenedBy     id.ID
	AcceptedBy   id.ID
	OpenedAt     time.Time
	AcceptedAt   time.Time
	UpdatedAt    time.Time
}

func (h *Handover) Normalize() {
	if h == nil {
		return
	}
	h.BusinessDate = strings.TrimSpace(h.BusinessDate)
	h.FromShift = clip(strings.TrimSpace(h.FromShift), MaxShiftNameLen)
	h.ToShift = clip(strings.TrimSpace(h.ToShift), MaxShiftNameLen)
	h.Note = clip(strings.TrimSpace(h.Note), MaxHandoverNote)
	h.Tasks = normalizeRefs(h.Tasks)
	h.Recoveries = normalizeRefs(h.Recoveries)
	h.Breakdowns = normalizeRefs(h.Breakdowns)
	if h.Status == "" {
		h.Status = HandoverOpen
	}
}

func (h Handover) Open() bool {
	return h.Status == HandoverOpen
}

func (h Handover) Validate(recordedShifts []string) error {
	if h.WorkspaceID == id.Zero() || h.OpenedBy == id.Zero() {
		return errx.Invalid("invalid_handover", "workspace and opener are required")
	}
	if _, err := ParseBusinessDate(h.BusinessDate); err != nil {
		return err
	}
	if h.Status != HandoverOpen && h.Status != HandoverAccepted {
		return errx.Invalid("invalid_handover_status", "handover must be open or accepted")
	}
	if err := shiftOrEmpty(h.FromShift, recordedShifts); err != nil {
		return err
	}
	if err := shiftOrEmpty(h.ToShift, recordedShifts); err != nil {
		return err
	}
	if utf8.RuneCountInString(h.Note) > MaxHandoverNote {
		return errx.Invalid("invalid_handover_note", "note is too long")
	}
	return nil
}

func OpenHandover(workspaceID, opener id.ID, date, from, to, note string, recordedShifts []string, tasks, recoveries, breakdowns []HandoverRef, now time.Time) (Handover, error) {
	h := Handover{
		ID:           id.New(),
		WorkspaceID:  workspaceID,
		BusinessDate: date,
		FromShift:    from,
		ToShift:      to,
		Status:       HandoverOpen,
		Note:         note,
		Tasks:        tasks,
		Recoveries:   recoveries,
		Breakdowns:   breakdowns,
		OpenedBy:     opener,
		OpenedAt:     now,
		UpdatedAt:    now,
	}
	h.Normalize()
	if err := h.Validate(recordedShifts); err != nil {
		return Handover{}, err
	}
	return h, nil
}

func (h *Handover) Accept(actor id.ID, now time.Time) error {
	if h.Status != HandoverOpen {
		return errx.Invalid("handover_closed", "this handover is already accepted")
	}
	if actor == id.Zero() {
		return errx.Invalid("invalid_handover", "incoming manager is required")
	}
	h.Status = HandoverAccepted
	h.AcceptedBy = actor
	h.AcceptedAt = now
	h.UpdatedAt = now
	return nil
}

func shiftOrEmpty(name string, recorded []string) error {
	if name == "" {
		return nil
	}
	if len(recorded) == 0 {
		return errx.Invalid("shift_not_recorded", "no shift is recorded. Do not invent one")
	}
	if !shiftKnown(recorded, name) {
		return errx.Invalid("unknown_shift", "shift is not on the cafe definition")
	}
	return nil
}

// RecoverySnapshotTitle is the recorded guest line. Kind is used only when there is no comment.
func RecoverySnapshotTitle(v Voice) string {
	if c := strings.TrimSpace(v.Comment); c != "" {
		return clip(c, MaxHandoverTitle)
	}
	return clip(string(v.Kind), MaxHandoverTitle)
}

func normalizeRefs(in []HandoverRef) []HandoverRef {
	out := make([]HandoverRef, 0, len(in))
	seen := map[id.ID]struct{}{}
	for _, r := range in {
		if r.ID == id.Zero() {
			continue
		}
		if _, ok := seen[r.ID]; ok {
			continue
		}
		seen[r.ID] = struct{}{}
		out = append(out, HandoverRef{ID: r.ID, Title: clip(strings.TrimSpace(r.Title), MaxHandoverTitle)})
	}
	return out
}
