package utils

import (
	"time"
)

// UTC converts time to UTC timezone
func UTC(t *time.Time) {
	if t.IsZero() {
		// to fix issue with timezones for tests
		*t = time.Time{}
		return
	}
	*t = t.UTC()
}
