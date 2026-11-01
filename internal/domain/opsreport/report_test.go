package opsreport_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/opsreport"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestReportAssignThenResolve(t *testing.T) {
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	r, err := opsreport.New(id.New(), id.New(), "", "Espresso machine jammed", "won't grind", now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != opsreport.KindBreakage || r.Status != opsreport.StatusOpen {
		t.Fatalf("got kind=%s status=%s", r.Kind, r.Status)
	}
	if err := r.Assign(id.New(), id.New(), now); err != nil {
		t.Fatal(err)
	}
	if r.Status != opsreport.StatusAssigned {
		t.Fatalf("status=%s", r.Status)
	}
	if err := r.Resolve(now); err != nil {
		t.Fatal(err)
	}
	if r.Open() {
		t.Fatal("resolved must not stay open")
	}
}

func TestRejectsBlankTitle(t *testing.T) {
	_, err := opsreport.New(id.New(), id.New(), "breakage", "  ", "", time.Now())
	if err == nil {
		t.Fatal("blank title must fail")
	}
}
