package finance_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestApproveDoesNotMoveMoney(t *testing.T) {
	now := time.Now().UTC()
	a, err := finance.NewApproval(id.New(), id.New(), "Milk invoice", "Al-Nahrain", "", "", 250000, now)
	if err != nil {
		t.Fatal(err)
	}
	if a.Currency != finance.CurrencyIQD || a.Status != finance.StatusOpen {
		t.Fatalf("got currency=%s status=%s", a.Currency, a.Status)
	}
	if err := a.Act(finance.ActionApprove, now); err != nil {
		t.Fatal(err)
	}
	if a.Status != finance.StatusApproved || a.Amount != 250000 {
		t.Fatalf("approve must only flip status, amount=%d status=%s", a.Amount, a.Status)
	}
}

func TestAmountChangeDropsApproval(t *testing.T) {
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
	if a.Status != finance.StatusOpen || a.Amount != 90000 {
		t.Fatalf("revised amount must reopen pack: status=%s amount=%d", a.Status, a.Amount)
	}
}
