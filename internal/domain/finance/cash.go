package finance

import (
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const CurrencyIQD = "IQD"

// Close is a GM-verified till count. It is not a POS sale and never invents a zero.
type Close struct {
	ID             id.ID
	WorkspaceID    id.ID
	BusinessDate   string
	CountedAmount  int64
	ExpectedAmount *int64
	Variance       int64
	Note           string
	ClosedByUserID id.ID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewClose(workspaceID, closer id.ID, date string, counted int64, expected *int64, note string, now time.Time) (Close, error) {
	c := Close{
		ID:             id.New(),
		WorkspaceID:    workspaceID,
		ClosedByUserID: closer,
		CreatedAt:      now,
	}
	if err := c.apply(date, counted, expected, note, now); err != nil {
		return Close{}, err
	}
	if workspaceID == id.Zero() || closer == id.Zero() {
		return Close{}, errx.Invalid("invalid_close_refs", "workspace and closer are required")
	}
	return c, nil
}

func (c *Close) Recount(counted int64, expected *int64, note string, now time.Time) error {
	return c.apply(c.BusinessDate, counted, expected, note, now)
}

func (c *Close) apply(date string, counted int64, expected *int64, note string, now time.Time) error {
	date = strings.TrimSpace(date)
	if date == "" {
		return errx.Invalid("invalid_date", "business date is required")
	}
	if counted < 0 {
		return errx.Invalid("invalid_amount", "counted amount cannot be negative")
	}
	var variance int64
	if expected != nil {
		if *expected < 0 {
			return errx.Invalid("invalid_amount", "expected amount cannot be negative")
		}
		copyExpected := *expected
		c.ExpectedAmount = &copyExpected
		variance = counted - copyExpected
	} else {
		c.ExpectedAmount = nil
	}
	c.BusinessDate = date
	c.CountedAmount = counted
	c.Variance = variance
	c.Note = strings.TrimSpace(note)
	c.UpdatedAt = now
	return nil
}

func (c Close) HasVariance() bool {
	return c.ExpectedAmount != nil && c.Variance != 0
}
