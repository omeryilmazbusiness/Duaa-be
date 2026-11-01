package finance_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestApproveDoesNotMoveMoney(t *testing.T) {
	now := time.Now().UTC()
	a, err := finance.NewApproval(id.New(), id.New(), "Milk invoice", "Al-Nahrain", "", "photo", 250000, now)
	if err != nil {
		t.Fatal(err)
	}
	if a.Currency != finance.CurrencyIQD || a.Status != finance.StatusOpen || a.Version != 1 {
		t.Fatalf("got currency=%s status=%s version=%d", a.Currency, a.Status, a.Version)
	}
	if err := a.Act(finance.ActionApprove, now); err != nil {
		t.Fatal(err)
	}
	if a.Status != finance.StatusApproved || a.Amount != 250000 {
		t.Fatalf("approve must only flip status, amount=%d status=%s", a.Amount, a.Status)
	}
}

func TestApproveRequiresEvidence(t *testing.T) {
	now := time.Now().UTC()
	a, err := finance.NewApproval(id.New(), id.New(), "Milk invoice", "Al-Nahrain", "", "", 250000, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Act(finance.ActionApprove, now); err == nil || !errors.Is(err, errx.ErrInvalid) {
		t.Fatalf("empty evidence must block approve: %v", err)
	}
	if a.Status != finance.StatusOpen || a.Amount != 250000 {
		t.Fatalf("failed approve must not change the pack: %+v", a)
	}
}

func TestNeedsEvidenceBlocksApproveUntilAttach(t *testing.T) {
	now := time.Now().UTC()
	a, err := finance.NewApproval(id.New(), id.New(), "Milk invoice", "Al-Nahrain", "", "old-slip", 250000, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Act(finance.ActionRequestEvidence, now); err != nil {
		t.Fatal(err)
	}
	if err := a.Act(finance.ActionApprove, now); err == nil {
		t.Fatal("needs_evidence must block approve even with old evidence")
	}
	if err := a.AttachEvidence("new-slip", now); err != nil {
		t.Fatal(err)
	}
	if a.Status != finance.StatusOpen || a.Version != 2 || a.Evidence != "new-slip" {
		t.Fatalf("attach must reopen and bump version: %+v", a)
	}
	if err := a.Act(finance.ActionApprove, now); err != nil {
		t.Fatal(err)
	}
	if a.Status != finance.StatusApproved || a.Amount != 250000 {
		t.Fatalf("approve after evidence must not move money: %+v", a)
	}
}

func TestClosedPackCannotAct(t *testing.T) {
	now := time.Now().UTC()
	a, err := finance.NewApproval(id.New(), id.New(), "Milk invoice", "Al-Nahrain", "", "photo", 250000, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Act(finance.ActionApprove, now); err != nil {
		t.Fatal(err)
	}
	if err := a.Act(finance.ActionReject, now); err == nil || !errors.Is(err, errx.ErrInvalid) {
		t.Fatalf("closed pack must not accept another decision: %v", err)
	}
	if err := a.AttachEvidence("later", now); err == nil {
		t.Fatal("closed pack must not take evidence")
	}
	if a.Status != finance.StatusApproved {
		t.Fatalf("closed pack stayed %s", a.Status)
	}
}

func TestAmountChangeDropsApprovalAndBumpsVersion(t *testing.T) {
	now := time.Now().UTC()
	a, err := finance.NewApproval(id.New(), id.New(), "Coffee beans", "Supplier", "IQD", "photo", 80000, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Act(finance.ActionApprove, now); err != nil {
		t.Fatal(err)
	}
	if err := a.ReviseAmount(90000, now); err != nil {
		t.Fatal(err)
	}
	if a.Status != finance.StatusOpen || a.Amount != 90000 || a.Version != 2 {
		t.Fatalf("revised amount must reopen pack: status=%s amount=%d version=%d", a.Status, a.Amount, a.Version)
	}
	want := finance.PackFingerprint("Coffee beans", "Supplier", 90000, "IQD")
	if a.Fingerprint != want {
		t.Fatalf("fingerprint=%s want=%s", a.Fingerprint, want)
	}
}

func TestDuplicateFingerprintIgnoresSpacingCase(t *testing.T) {
	now := time.Now().UTC()
	wid := id.New()
	first, err := finance.NewApproval(wid, id.New(), "Milk invoice", "Al-Nahrain", "IQD", "photo", 250000, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := finance.NewApproval(wid, id.New(), "  MILK   invoice ", "al-nahrain", "iqd", "other", 250000, now)
	if err != nil {
		t.Fatal(err)
	}
	if !finance.PendingDuplicate([]finance.Approval{first}, second) {
		t.Fatal("same invoice must collide while pending")
	}
	if err := first.Act(finance.ActionApprove, now); err != nil {
		t.Fatal(err)
	}
	if finance.PendingDuplicate([]finance.Approval{first}, second) {
		t.Fatal("decided pack must not block a new one")
	}
}

func TestEvidenceTooLong(t *testing.T) {
	now := time.Now().UTC()
	if _, err := finance.NewApproval(id.New(), id.New(), "Milk", "S", "", strings.Repeat("x", finance.MaxEvidence+1), 1, now); err == nil {
		t.Fatal("long evidence")
	}
}
