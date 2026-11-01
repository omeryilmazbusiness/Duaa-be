package executive_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/executive"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestDecisionHappyPath(t *testing.T) {
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	d, err := executive.NewDecision(id.New(), id.New(), "late_staff", "overdue=3", "late_staff", "brief.overdueTasks", "3", "high", "ops", now)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Can(executive.ActionAcknowledge) {
		t.Fatal("open can be acknowledged")
	}
	snooze := executive.EndOfLocalDay(now, "Asia/Baghdad")
	if err := d.Apply(executive.ActionAcknowledge, now, snooze); err != nil {
		t.Fatal(err)
	}
	if d.Status != executive.StatusAcknowledged {
		t.Fatalf("status=%s", d.Status)
	}
	if err := d.Apply(executive.ActionAssign, now, snooze); err != nil {
		t.Fatal(err)
	}
	if d.Status != executive.StatusAssigned {
		t.Fatalf("status=%s", d.Status)
	}
	if err := d.Apply(executive.ActionResolve, now, snooze); err != nil {
		t.Fatal(err)
	}
	if d.Status != executive.StatusResolved {
		t.Fatalf("status=%s", d.Status)
	}
	if !d.HiddenBySnooze("overdue=3", now.Add(time.Hour)) {
		t.Fatal("resolved same fingerprint is hidden until local midnight")
	}
	if d.HiddenBySnooze("overdue=4", now.Add(time.Hour)) {
		t.Fatal("changed fingerprint must surface again")
	}
	if d.HiddenBySnooze("overdue=3", snooze.Add(time.Second)) {
		t.Fatal("after snooze the card returns")
	}
}

func TestRejectsEmptySource(t *testing.T) {
	_, err := executive.NewDecision(id.New(), id.New(), "  ", "", "", "", "", "", "", time.Now().UTC())
	if err == nil {
		t.Fatal("expected invalid source")
	}
}

func TestParseAction(t *testing.T) {
	if _, err := executive.ParseAction("snooze"); err == nil {
		t.Fatal("unknown action")
	}
	got, err := executive.ParseAction("assign")
	if err != nil || got != executive.ActionAssign {
		t.Fatalf("got %s err=%v", got, err)
	}
}

func TestEndOfLocalDayBaghdad(t *testing.T) {
	now := time.Date(2026, 10, 29, 20, 30, 0, 0, time.UTC) // 23:30 in Baghdad
	end := executive.EndOfLocalDay(now, "Asia/Baghdad")
	loc, err := time.LoadLocation("Asia/Baghdad")
	if err != nil {
		t.Fatal(err)
	}
	local := end.In(loc)
	if local.Day() != 30 || local.Hour() != 0 {
		t.Fatalf("end=%s local=%s", end, local)
	}
}
