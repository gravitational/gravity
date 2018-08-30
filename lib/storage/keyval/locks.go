/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
