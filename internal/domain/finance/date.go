package finance

import "time"

// BusinessDate is the cafe's local calendar day for till close and dashboard tiles.
func BusinessDate(now time.Time, timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil || loc == nil {
		loc = time.UTC
	}
	return now.In(loc).Format("2006-01-02")
}
