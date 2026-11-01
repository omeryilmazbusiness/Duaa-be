package executive

import "github.com/dua-os/dua-os-be/internal/domain/tenancy"

// Card is the GM morning queue item. Identity is the live source, not a stored brief.
type Card struct {
	SourceKey      string
	Fingerprint    string
	Kind           tenancy.SignalKind
	Severity       string
	TitleKey       string
	Detail         string
	Count          int
	Priority       int
	TargetTab      tenancy.Tab
	AllowedActions []string
	Decision       *Decision
}

type Brief struct {
	GeneratedAt  string
	Date         string
	Kind         tenancy.Kind
	Headline     Headline
	Pulse        Pulse
	Cards        []Card
	Sources      []Source
	SnoozedCount int
	IsClear      bool
}
