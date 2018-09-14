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

// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Package run provides synchronization, error propagation, and Context
// cancelation for groups of goroutines working on subtasks of a common task.
// The package is based on golang.org/x/sync/errgroup and adds concurrency limits
// on top.
package run

import (
	"context"
	"runtime"
	"sync"
)

// A Group is a collection of goroutines working on subtasks that are part of
// the same overall task.
// The parallelization of tasks is controlled by a semaphore that is either
// unrestricted (allows unlimited number of concurrent tasks) or limits
// them to a certain amount subject to implemented strategy.
//
// A zero Group is valid and does not cancel on error.
type Group struct {
	cancel  func()
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
	semaphoreStore
}

// WithContext returns a new group with the specified concurrency configuration.
func WithContext(ctx context.Context, options ...Option) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	group := &Group{
		cancel: cancel,
	}
	for _, opt := range options {
		opt(group)
	}
	return group, ctx
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (r *Group) Wait() error {
	r.wg.Wait()
	if r.cancel != nil {
		r.cancel()
	}
	return r.err
}

// Go calls the given function in a new goroutine.
// The call to Go might block if there're already as many tasks
// running as configured by r.parallel.
//
// The first call to return a non-nil error cancels the group; its error will be
// returned by Wait.
func (r *Group) Go(ctx context.Context, fn func() error) {
	r.alloc(ctx)

	r.wg.Add(1)
	go func() {
		defer func() {
			r.wg.Done()
			r.free(ctx)
		}()
		if err := fn(); err != nil {
			r.errOnce.Do(func() {
				r.err = err
				if r.cancel != nil {
					r.cancel()
				}
			})
		}
	}()
}

// Option is a configuration option for Group
type Option func(group *Group)

type semaphore interface {
	alloc(context.Context)
	free(context.Context)
}

func (r chanSemaphore) alloc(ctx context.Context) {
	select {
	case r <- struct{}{}:
	case <-ctx.Done():
		return
	}
}

func (r chanSemaphore) free(ctx context.Context) {
	select {
	case <-r:
	case <-ctx.Done():
	}
}

type chanSemaphore chan struct{}

func (r semaphoreStore) alloc(ctx context.Context) {
	if r.semaphore != nil {
		r.semaphore.alloc(ctx)
	}
}

func (r semaphoreStore) free(ctx context.Context) {
	if r.semaphore != nil {
		r.semaphore.free(ctx)
	}
}

// semaphoreStore wraps a semaphore implementation.
// It implements semaphore and does nothing if the underlying semaphore
// has not been initialized
type semaphoreStore struct {
	semaphore
}

// WithCPU creates a new semaphore that executes as many tasks as there're
// CPU cores
func WithCPU() Option {
	return func(group *Group) {
		group.semaphore = make(chanSemaphore, runtime.NumCPU())
	}
}

// WithParallel creates a new semaphore that caps the number of tasks to the
// specified value.
//
// If parallel < 0, then the tasks are not capped.
// If parallel == 0, then the behaviour is as with parallel == 1
// If parallel > 0, then the specified number of tasks is allowed to run concurrently
func WithParallel(parallel int) Option {
	return func(group *Group) {
		if parallel < 0 {
			// No explicit semaphore
			return
		}

		switch parallel {
		case 0, 1:
			group.semaphore = make(chanSemaphore, 1)
		default:
			group.semaphore = make(chanSemaphore, parallel)
		}
	}
}
