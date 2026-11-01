package brief

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestComposeHidesUnavailableSources(t *testing.T) {
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	pulse := executive.Pulse{OverdueTasks: 3, OpenTasks: 5, CompletedToday: 1}
	signals := []executive.Signal{
		{
			Kind: tenancy.SignalLateStaff, SourceKey: "late_staff", Fingerprint: "overdue=3",
			Available: true, Severity: "high", TitleKey: "brief.overdueTasks", Detail: "3",
			Count: 3, Priority: 10, TargetTab: tenancy.TabOps, Pulse: &pulse,
		},
		{Kind: tenancy.SignalCustomerVoice, Available: false, ReasonKey: "brief.sourceNotWired"},
	}
	brief := buildBrief(tenancy.KindCafe, now, signals, nil)
	if brief.IsClear {
		t.Fatal("overdue work is not all-clear")
	}
	if len(brief.Cards) != 1 || brief.Cards[0].Kind != tenancy.SignalLateStaff {
		t.Fatalf("cards=%v", brief.Cards)
	}
	if brief.Headline.PendingDecisions != 1 || brief.Headline.OverdueTasks != 3 {
		t.Fatalf("headline=%v", brief.Headline)
	}
	if len(brief.Sources) != 2 || brief.Sources[1].Status != executive.SourceUnavailable {
		t.Fatalf("sources=%v", brief.Sources)
	}
	for _, c := range brief.Cards {
		if c.Kind == tenancy.SignalCustomerVoice {
			t.Fatal("unwired reviews must not become a fake card")
		}
	}
}

func TestComposeCashVarianceRaisesDashboardCard(t *testing.T) {
	now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	signals := []executive.Signal{
		{
			Kind: tenancy.SignalCashVariance, SourceKey: "cash_variance", Fingerprint: "variance=-2000",
			Available: true, Severity: "high", TitleKey: "brief.cashVariance", Detail: "-2000",
			Count: 1, Priority: 20, TargetTab: tenancy.TabDashboard,
		},
		{Kind: tenancy.SignalCustomerVoice, Available: false, ReasonKey: "brief.sourceNotWired"},
	}
	brief := buildBrief(tenancy.KindCafe, now, signals, nil)
	if len(brief.Cards) != 1 || brief.Cards[0].Kind != tenancy.SignalCashVariance {
		t.Fatalf("cards=%v", brief.Cards)
	}
	if brief.Cards[0].TargetTab != tenancy.TabDashboard {
		t.Fatalf("cash card must open dashboard, got %s", brief.Cards[0].TargetTab)
	}
}

func TestComposeClosedTillWithoutVarianceIsNotACard(t *testing.T) {
	now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	signals := []executive.Signal{
		{
			Kind: tenancy.SignalCashVariance, SourceKey: "cash_variance", Fingerprint: "variance=0",
			Available: true, Count: 0,
		},
	}
	brief := buildBrief(tenancy.KindCafe, now, signals, nil)
	if len(brief.Cards) != 0 {
		t.Fatalf("balanced till must not raise a card: %v", brief.Cards)
	}
	if brief.Sources[0].Status != executive.SourceLive {
		t.Fatal("closed till is a live source even with no variance")
	}
}

func TestComposeSnoozeSameFingerprint(t *testing.T) {
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	until := now.Add(12 * time.Hour)
	d, err := executive.NewDecision(id.New(), id.New(), "late_staff", "overdue=3", "late_staff", "brief.overdueTasks", "3", "high", "ops", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(executive.ActionResolve, now, until); err != nil {
		t.Fatal(err)
	}
	pulse := executive.Pulse{OverdueTasks: 3, OpenTasks: 3}
	signals := []executive.Signal{{
		Kind: tenancy.SignalLateStaff, SourceKey: "late_staff", Fingerprint: "overdue=3",
		Available: true, Count: 3, Pulse: &pulse,
	}}
	brief := buildBrief(tenancy.KindCafe, now, signals, []executive.Decision{d})
	if len(brief.Cards) != 0 {
		t.Fatalf("snoozed card still visible: %v", brief.Cards)
	}
	if brief.SnoozedCount != 1 {
		t.Fatalf("snoozed=%d", brief.SnoozedCount)
	}
}

func TestComposeFingerprintChangeUnsnoozes(t *testing.T) {
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	until := now.Add(12 * time.Hour)
	d, err := executive.NewDecision(id.New(), id.New(), "late_staff", "overdue=3", "late_staff", "brief.overdueTasks", "3", "high", "ops", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(executive.ActionResolve, now, until); err != nil {
		t.Fatal(err)
	}
	pulse := executive.Pulse{OverdueTasks: 4, OpenTasks: 4}
	signals := []executive.Signal{{
		Kind: tenancy.SignalLateStaff, SourceKey: "late_staff", Fingerprint: "overdue=4",
		Available: true, Count: 4, Pulse: &pulse,
	}}
	brief := buildBrief(tenancy.KindCafe, now, signals, []executive.Decision{d})
	if len(brief.Cards) != 1 {
		t.Fatalf("changed fingerprint must return, cards=%d", len(brief.Cards))
	}
}

func TestAllClearHasNoCards(t *testing.T) {
	now := time.Now().UTC()
	pulse := executive.Pulse{}
	signals := []executive.Signal{{
		Kind: tenancy.SignalLateStaff, SourceKey: "late_staff", Fingerprint: "overdue=0",
		Available: true, Pulse: &pulse,
	}}
	brief := buildBrief(tenancy.KindCafe, now, signals, nil)
	if !brief.IsClear || len(brief.Cards) != 0 {
		t.Fatalf("clear=%v cards=%d", brief.IsClear, len(brief.Cards))
	}
}

func TestCafeFloorPulseMergesWithoutInventingWork(t *testing.T) {
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	tasks := executive.Pulse{OverdueTasks: 1, OpenTasks: 3, CompletedToday: 2}
	signals := []executive.Signal{
		{Kind: tenancy.SignalLateStaff, SourceKey: "late_staff", Fingerprint: "overdue=1", Available: true, Count: 1, Priority: 10, Pulse: &tasks},
		{Kind: tenancy.SignalHandover, SourceKey: "handover", Fingerprint: "open=1", Available: true, Count: 1, Priority: 5, TitleKey: "brief.handover", TargetTab: tenancy.TabFeedback, Pulse: &executive.Pulse{HandoverOpen: true}},
		{Kind: tenancy.SignalBreakdown, SourceKey: "breakdown", Fingerprint: "open=2", Available: true, Count: 2, Priority: 14, TitleKey: "brief.breakdowns", TargetTab: tenancy.TabFeedback, Pulse: &executive.Pulse{OpenBreakdowns: 2}},
		{Kind: tenancy.SignalDailyClose, SourceKey: "daily_close", Fingerprint: "written=0", Available: true, Count: 0, Pulse: &executive.Pulse{IntakeWritten: false}},
		{Kind: tenancy.SignalCustomerVoice, SourceKey: "customer_voice", Fingerprint: "open=0,late=0", Available: true, Count: 0, Pulse: &executive.Pulse{}},
	}
	brief := buildBrief(tenancy.KindCafe, now, signals, nil)
	if brief.Pulse.OpenTasks != 3 || !brief.Pulse.HandoverOpen || brief.Pulse.OpenBreakdowns != 2 || brief.Pulse.IntakeWritten || brief.Pulse.OpenRecoveries != 0 {
		t.Fatalf("merged pulse=%+v", brief.Pulse)
	}
	if len(brief.Cards) != 3 {
		t.Fatalf("handover+late+breakdown cards, got %d", len(brief.Cards))
	}
	if brief.Cards[0].Kind != tenancy.SignalHandover {
		t.Fatalf("handover is first: %v", brief.Cards[0].Kind)
	}
}

func TestEmptyHandoverAndCloseDoNotRaiseCards(t *testing.T) {
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	signals := []executive.Signal{
		{Kind: tenancy.SignalHandover, Available: true, Count: 0, Pulse: &executive.Pulse{HandoverOpen: false}},
		{Kind: tenancy.SignalDailyClose, Available: true, Count: 0, Pulse: &executive.Pulse{IntakeWritten: false}},
		{Kind: tenancy.SignalBreakdown, Available: true, Count: 0, Pulse: &executive.Pulse{}},
	}
	brief := buildBrief(tenancy.KindCafe, now, signals, nil)
	if len(brief.Cards) != 0 || !brief.IsClear {
		t.Fatalf("quiet floor must stay clear: cards=%d clear=%v", len(brief.Cards), brief.IsClear)
	}
}
