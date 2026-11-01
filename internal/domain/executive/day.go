package executive

import "time"

// EndOfLocalDay is the exclusive snooze boundary for a GM "done for today" action.
func EndOfLocalDay(now time.Time, timezone string) time.Time {
	loc, err := time.LoadLocation(timezone)
	if err != nil || loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	y, m, d := local.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, loc).UTC()
}
