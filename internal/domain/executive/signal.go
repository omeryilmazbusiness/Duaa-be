package executive

import "github.com/dua-os/dua-os-be/internal/domain/tenancy"

const (
	SourceLive        = "live"
	SourceUnavailable = "unavailable"
)

// Signal is one live input to the morning brief. Unavailable signals stay honest:
// they appear as a missing source, never as a fake card.
type Signal struct {
	Kind        tenancy.SignalKind
	SourceKey   string
	Fingerprint string
	Available   bool
	ReasonKey   string
	Severity    string
	TitleKey    string
	Detail      string
	Count       int
	Priority    int
	TargetTab   tenancy.Tab
	Pulse       *Pulse
}

func (s Signal) RaisesCard() bool {
	return s.Available && s.Count > 0
}

type Pulse struct {
	OverdueTasks   int
	OpenTasks      int
	CompletedToday int
	OpenRecoveries int
	LateRecoveries int
	OpenBreakdowns int
	HandoverOpen   bool
	IntakeWritten  bool
}

type Source struct {
	Kind      tenancy.SignalKind
	Status    string
	ReasonKey string
}

type Headline struct {
	OverdueTasks      int
	PendingDecisions  int
	AssignedFollowups int
}
