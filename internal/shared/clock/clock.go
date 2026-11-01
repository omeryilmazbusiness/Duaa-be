package clock

import "time"

type Clock interface {
	Now() time.Time
}

type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

type Frozen struct{ T time.Time }

func (f Frozen) Now() time.Time { return f.T }
