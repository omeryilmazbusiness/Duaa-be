package brief

import (
	"context"
	"sort"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	ports     map[tenancy.SignalKind]Port
	decisions DecisionReader
	clock     clock.Clock
}

func New(decisions DecisionReader, clk clock.Clock, ports ...Port) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	s := &Service{
		ports:     make(map[tenancy.SignalKind]Port, len(ports)),
		decisions: decisions,
		clock:     clk,
	}
	for _, p := range ports {
		s.ports[p.Kind()] = p
	}
	return s
}

type DecisionView struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	AssigneeUserID string `json:"assigneeUserId,omitempty"`
	LinkedTaskID   string `json:"linkedTaskId,omitempty"`
}

type CardView struct {
	SourceKey      string        `json:"sourceKey"`
	Fingerprint    string        `json:"fingerprint"`
	Kind           string        `json:"kind"`
	Severity       string        `json:"severity"`
	TitleKey       string        `json:"titleKey"`
	Detail         string        `json:"detail,omitempty"`
	Count          int           `json:"count"`
	Priority       int           `json:"priority"`
	TargetTab      string        `json:"targetTab"`
	AllowedActions []string      `json:"allowedActions"`
	Decision       *DecisionView `json:"decision"`
}

type SourceView struct {
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	ReasonKey string `json:"reasonKey,omitempty"`
}

type HeadlineView struct {
	OverdueTasks      int `json:"overdueTasks"`
	PendingDecisions  int `json:"pendingDecisions"`
	AssignedFollowups int `json:"assignedFollowups"`
}

type PulseView struct {
	OverdueTasks   int  `json:"overdueTasks"`
	OpenTasks      int  `json:"openTasks"`
	CompletedToday int  `json:"completedToday"`
	OpenRecoveries int  `json:"openRecoveries"`
	LateRecoveries int  `json:"lateRecoveries"`
	OpenBreakdowns int  `json:"openBreakdowns"`
	HandoverOpen   bool `json:"handoverOpen"`
	IntakeWritten  bool `json:"intakeWritten"`
}

type View struct {
	GeneratedAt  string       `json:"generatedAt"`
	Date         string       `json:"date"`
	Kind         string       `json:"workspaceKind"`
	Headline     HeadlineView `json:"headline"`
	Pulse        PulseView    `json:"pulse"`
	Cards        []CardView   `json:"cards"`
	Sources      []SourceView `json:"sources"`
	SnoozedCount int          `json:"snoozedCount"`
	IsClear      bool         `json:"isClear"`
}

func (s *Service) ForGM(ctx context.Context, ac actor.Context) (View, error) {
	if ac.Scope != staffscope.GeneralManager && ac.Scope != staffscope.DepartmentManager {
		return View{}, errx.Forbidden("forbidden", "brief is a manager surface")
	}
	if !rbac.Allows(ac.Scope, rbac.ViewDashboard) {
		return View{}, errx.Forbidden("forbidden", "cannot view brief")
	}
	now := s.clock.Now()
	opts := CollectOpts{
		WorkspaceID: ac.Workspace.ID,
		Department:  staffscope.DepartmentScope(ac.Actor, ac.Catalog),
		Now:         now,
		Timezone:    ac.Workspace.Timezone,
	}
	wanted := ac.Catalog.ExecutiveSignals()
	signals := make([]executive.Signal, 0, len(wanted))
	for _, kind := range wanted {
		port, ok := s.ports[kind]
		if !ok {
			signals = append(signals, executive.Signal{
				Kind:      kind,
				Available: false,
				ReasonKey: "brief.sourceNotWired",
			})
			continue
		}
		sig, err := port.Collect(ctx, opts)
		if err != nil {
			return View{}, err
		}
		signals = append(signals, sig)
	}
	var stored []executive.Decision
	if s.decisions != nil {
		list, err := s.decisions.ListDecisions(ctx, ac.Workspace.ID)
		if err != nil {
			return View{}, err
		}
		stored = list
	}
	return toView(buildBrief(ac.Workspace.Kind, now, signals, stored)), nil
}

func buildBrief(kind tenancy.Kind, now time.Time, signals []executive.Signal, stored []executive.Decision) executive.Brief {
	pulse := executive.Pulse{}
	sources := make([]executive.Source, 0, len(signals))
	cards := make([]executive.Card, 0)
	snoozed := 0
	for _, sig := range signals {
		if !sig.Available {
			sources = append(sources, executive.Source{
				Kind:      sig.Kind,
				Status:    executive.SourceUnavailable,
				ReasonKey: sig.ReasonKey,
			})
			continue
		}
		sources = append(sources, executive.Source{Kind: sig.Kind, Status: executive.SourceLive})
		applyPulse(&pulse, sig)
		if !sig.RaisesCard() {
			continue
		}
		match := matchDecision(stored, sig.SourceKey)
		if match != nil && match.HiddenBySnooze(sig.Fingerprint, now) {
			snoozed++
			continue
		}
		status := executive.StatusOpen
		var live *executive.Decision
		if match != nil && match.Active() {
			copy := *match
			live = &copy
			status = copy.Status
		}
		cards = append(cards, executive.Card{
			SourceKey:      sig.SourceKey,
			Fingerprint:    sig.Fingerprint,
			Kind:           sig.Kind,
			Severity:       sig.Severity,
			TitleKey:       sig.TitleKey,
			Detail:         sig.Detail,
			Count:          sig.Count,
			Priority:       sig.Priority,
			TargetTab:      sig.TargetTab,
			AllowedActions: executive.AllowedActions(status),
			Decision:       live,
		})
	}
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].Priority < cards[j].Priority })
	pending, assigned := 0, 0
	for _, c := range cards {
		if c.Decision == nil || c.Decision.Status == executive.StatusOpen {
			pending++
			continue
		}
		if c.Decision.Status == executive.StatusAssigned {
			assigned++
		}
	}
	return executive.Brief{
		GeneratedAt: now.Format(time.RFC3339),
		Date:        now.Format("2006-01-02"),
		Kind:        kind,
		Headline: executive.Headline{
			OverdueTasks:      pulse.OverdueTasks,
			PendingDecisions:  pending,
			AssignedFollowups: assigned,
		},
		Pulse:        pulse,
		Cards:        cards,
		Sources:      sources,
		SnoozedCount: snoozed,
		IsClear: pulse.OverdueTasks == 0 && pulse.OpenTasks == 0 && !pulse.HandoverOpen &&
			pulse.OpenRecoveries == 0 && pulse.OpenBreakdowns == 0 && len(cards) == 0,
	}
}

func applyPulse(dst *executive.Pulse, sig executive.Signal) {
	if sig.Pulse == nil {
		return
	}
	p := *sig.Pulse
	switch sig.Kind {
	case tenancy.SignalLateStaff:
		dst.OverdueTasks, dst.OpenTasks, dst.CompletedToday = p.OverdueTasks, p.OpenTasks, p.CompletedToday
	case tenancy.SignalCustomerVoice:
		dst.OpenRecoveries, dst.LateRecoveries = p.OpenRecoveries, p.LateRecoveries
	case tenancy.SignalBreakdown:
		dst.OpenBreakdowns = p.OpenBreakdowns
	case tenancy.SignalHandover:
		dst.HandoverOpen = p.HandoverOpen
	case tenancy.SignalDailyClose:
		dst.IntakeWritten = p.IntakeWritten
	}
}

func matchDecision(list []executive.Decision, sourceKey string) *executive.Decision {
	var resolved *executive.Decision
	for i := range list {
		d := list[i]
		if d.SourceKey != sourceKey {
			continue
		}
		if d.Active() {
			return &list[i]
		}
		if d.Status == executive.StatusResolved {
			if resolved == nil || d.UpdatedAt.After(resolved.UpdatedAt) {
				resolved = &list[i]
			}
		}
	}
	return resolved
}

func toView(b executive.Brief) View {
	cards := make([]CardView, 0, len(b.Cards))
	for _, c := range b.Cards {
		cards = append(cards, CardView{
			SourceKey:      c.SourceKey,
			Fingerprint:    c.Fingerprint,
			Kind:           string(c.Kind),
			Severity:       c.Severity,
			TitleKey:       c.TitleKey,
			Detail:         c.Detail,
			Count:          c.Count,
			Priority:       c.Priority,
			TargetTab:      string(c.TargetTab),
			AllowedActions: c.AllowedActions,
			Decision:       decisionView(c.Decision),
		})
	}
	sources := make([]SourceView, 0, len(b.Sources))
	for _, src := range b.Sources {
		sources = append(sources, SourceView{Kind: string(src.Kind), Status: src.Status, ReasonKey: src.ReasonKey})
	}
	return View{
		GeneratedAt: b.GeneratedAt,
		Date:        b.Date,
		Kind:        string(b.Kind),
		Headline: HeadlineView{
			OverdueTasks:      b.Headline.OverdueTasks,
			PendingDecisions:  b.Headline.PendingDecisions,
			AssignedFollowups: b.Headline.AssignedFollowups,
		},
		Pulse: PulseView{
			OverdueTasks:   b.Pulse.OverdueTasks,
			OpenTasks:      b.Pulse.OpenTasks,
			CompletedToday: b.Pulse.CompletedToday,
			OpenRecoveries: b.Pulse.OpenRecoveries,
			LateRecoveries: b.Pulse.LateRecoveries,
			OpenBreakdowns: b.Pulse.OpenBreakdowns,
			HandoverOpen:   b.Pulse.HandoverOpen,
			IntakeWritten:  b.Pulse.IntakeWritten,
		},
		Cards:        cards,
		Sources:      sources,
		SnoozedCount: b.SnoozedCount,
		IsClear:      b.IsClear,
	}
}

func decisionView(d *executive.Decision) *DecisionView {
	if d == nil {
		return nil
	}
	v := &DecisionView{ID: d.ID.String(), Status: d.Status}
	if d.AssigneeUserID != id.Zero() {
		v.AssigneeUserID = d.AssigneeUserID.String()
	}
	if d.LinkedTaskID != id.Zero() {
		v.LinkedTaskID = d.LinkedTaskID.String()
	}
	return v
}
