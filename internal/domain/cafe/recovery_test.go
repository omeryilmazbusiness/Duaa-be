package cafe_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestNeedsRecoveryDoesNotInventHappyReviews(t *testing.T) {
	if (cafe.Voice{Kind: cafe.VoiceReview, Rating: 5}).NeedsRecovery() {
		t.Fatal("a 5-star review is not a recovery file")
	}
	if !(cafe.Voice{Kind: cafe.VoiceReview, Rating: 2}).NeedsRecovery() {
		t.Fatal("a low score opens a file")
	}
	if !(cafe.Voice{Kind: cafe.VoiceRequest, Comment: "water"}).NeedsRecovery() {
		t.Fatal("a request needs the floor")
	}
}

func TestRecoveryAssignCompensateCloseNeverPays(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	rec, err := cafe.OpenRecovery(id.New(), id.New(), now)
	if err != nil || rec.Status != cafe.RecoveryOpen {
		t.Fatalf("open=%+v err=%v", rec, err)
	}
	if err := rec.Compensate(10, "USD", 0, "", now); err == nil {
		t.Fatal("no invented compensation limit")
	}
	if err := rec.Compensate(50, "USD", 20, "USD", now); err == nil {
		t.Fatal("over limit must fail")
	}
	if err := rec.Compensate(10, "EUR", 20, "USD", now); err == nil {
		t.Fatal("currency must match SLA")
	}
	if err := rec.Compensate(10, "usd", 20, "USD", now); err != nil || rec.CompensationAmount != 10 || rec.CompensationCurrency != "USD" {
		t.Fatalf("compensate=%+v err=%v", rec, err)
	}
	if rec.Status != cafe.RecoveryOpen {
		t.Fatal("compensation records a gesture; it does not close or pay")
	}
	if err := rec.Assign(id.New(), id.New(), now); err != nil || rec.Status != cafe.RecoveryAssigned {
		t.Fatalf("assign=%+v err=%v", rec, err)
	}
	if err := rec.Close(now.Add(time.Minute), "tea replaced"); err != nil || rec.Status != cafe.RecoveryClosed {
		t.Fatalf("close=%+v err=%v", rec, err)
	}
	if err := rec.Assign(id.New(), id.Zero(), now); err == nil {
		t.Fatal("closed file stays closed")
	}
}

func TestRecoveryClockStaysHonestWhenSLAIsZero(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	rec, _ := cafe.OpenRecovery(id.New(), id.New(), now)
	clock := rec.Clock(cafe.SLA{}, now, now.Add(time.Hour))
	if clock.LateAck || clock.LateResolve || !clock.AckDue.IsZero() {
		t.Fatalf("empty SLA must not invent a deadline: %+v", clock)
	}
	clock = rec.Clock(cafe.SLA{AckMinutes: 15, ResolveMinutes: 60}, now, now.Add(20*time.Minute))
	if !clock.LateAck || clock.LateResolve {
		t.Fatalf("ack late only: %+v", clock)
	}
}
