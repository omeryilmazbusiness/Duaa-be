package work_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestCompleteAndOverdue(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	task, err := work.NewTask(id.New(), id.New(), id.New(), "Open bar", "", now, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if task.Overdue(now.Add(2 * time.Hour)) != true {
		t.Fatal("should be overdue")
	}
	if err := task.Complete(now.Add(30 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if task.Overdue(now.Add(2 * time.Hour)) {
		t.Fatal("completed is not overdue")
	}
}

func TestRejectsInvertedSchedule(t *testing.T) {
	now := time.Now().UTC()
	_, err := work.NewTask(id.New(), id.New(), id.New(), "x", "", now, now, now)
	if err == nil {
		t.Fatal("expected invalid schedule")
	}
}
