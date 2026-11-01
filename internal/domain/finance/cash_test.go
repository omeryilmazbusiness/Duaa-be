package finance_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestCloseVarianceFromExpected(t *testing.T) {
	now := time.Date(2026, 10, 30, 21, 0, 0, 0, time.UTC)
	expected := int64(100000)
	c, err := finance.NewClose(id.New(), id.New(), "2026-10-30", 98000, &expected, "short", now)
	if err != nil {
		t.Fatal(err)
	}
	if !c.HasVariance() || c.Variance != -2000 {
		t.Fatalf("variance=%d has=%v", c.Variance, c.HasVariance())
	}
}

func TestCloseWithoutExpectedHasNoVarianceCard(t *testing.T) {
	c, err := finance.NewClose(id.New(), id.New(), "2026-10-30", 50000, nil, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if c.HasVariance() {
		t.Fatal("no expected amount means no invented variance")
	}
}

func TestRejectsNegativeCount(t *testing.T) {
	_, err := finance.NewClose(id.New(), id.New(), "2026-10-30", -1, nil, "", time.Now())
	if err == nil {
		t.Fatal("negative counted amount must fail")
	}
}
