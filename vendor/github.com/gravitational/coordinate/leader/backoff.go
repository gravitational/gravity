package leader

import (
	"time"

	"github.com/cenkalti/backoff"
)

// NewUnlimitedExponentialBackOff returns a new exponential backoff interval
// w/o time limit
func NewUnlimitedExponentialBackOff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0
	b.MaxInterval = 10 * time.Second
	return b
}
