package keyval

import (
	"time"

	"github.com/gravitational/trace"
)

// AcquireLock grabs a lock that will be released automatically in ttl time
// blocks until lock is available
func (b *backend) AcquireLock(token string, ttl time.Duration) error {
	err := b.acquireLock(b.key(locksP, token), ttl)
	return trace.Wrap(err)
}

// TryAcquireLock grabs a lock that will be released automatically in ttl time
// tries once and either succeeds right away or fails
func (b *backend) TryAcquireLock(token string, ttl time.Duration) error {
	err := b.tryAcquireLock(b.key(locksP, token), ttl)
	return trace.Wrap(err)
}

// ReleaseLock releases lock by token name
func (b *backend) ReleaseLock(token string) error {
	err := b.releaseLock(b.key(locksP, token))
	return trace.Wrap(err)
}
