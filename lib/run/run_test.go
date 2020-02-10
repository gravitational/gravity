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
package run

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
)

func TestZeroGroup(t *testing.T) {
	err1 := errors.New("run_test: 1")
	err2 := errors.New("run_test: 2")

	cases := []struct {
		errs []error
	}{
		{errs: []error{}},
		{errs: []error{nil}},
		{errs: []error{err1}},
		{errs: []error{err1, nil}},
		{errs: []error{err1, nil, err2}},
	}

	ctx := context.Background()
	for _, tc := range cases {
		var g Group

		var firstErr error
		for i, err := range tc.errs {
			err := err
			g.Go(ctx, func() error { return err })

			if firstErr == nil && err != nil {
				firstErr = err
			}

			if gErr := g.Wait(); gErr != firstErr {
				t.Errorf("after Group.Go(func() error { return err }) for err in %v\n"+
					"g.Wait() = %v; want %v",
					tc.errs[:i+1], err, firstErr)
			}
		}
	}
}

func TestWithContext(t *testing.T) {
	errDoom := errors.New("group_test: doomed")

	cases := []struct {
		errs []error
		want error
	}{
		{want: nil},
		{errs: []error{nil}, want: nil},
		{errs: []error{errDoom}, want: errDoom},
		{errs: []error{errDoom, nil}, want: errDoom},
	}

	for _, tc := range cases {
		g, ctx := WithContext(context.Background())

		for _, err := range tc.errs {
			err := err
			g.Go(ctx, func() error { return err })
		}

		if err := g.Wait(); err != tc.want {
			t.Errorf("after %T.Go(func() error { return err }) for err in %v\n"+
				"g.Wait() = %v; want %v",
				g, tc.errs, err, tc.want)
		}

		canceled := false
		select {
		case <-ctx.Done():
			canceled = true
		default:
		}
		if !canceled {
			t.Errorf("after %T.Go(func() error { return err }) for err in %v\n"+
				"ctx.Done() was not closed",
				g, tc.errs)
		}
	}
}

func TestWithLimit(t *testing.T) {
	g, ctx := WithContext(context.Background(), WithParallel(1))

	var store slice
	for _, text := range []string{"first", "second", "third"} {
		text := text
		g.Go(ctx, func() error {
			_, err := store.append(text)
			return err
		})
	}

	if err := g.Wait(); err != nil {
		t.Errorf("Expected no errors but got %v.", err)
	}

	if store.String() != "firstsecondthird" {
		t.Errorf("Expected appends in a sequence but got %q.", store.String())
	}
}

func (r *slice) append(s string) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Buffer.WriteString(s)
}

type slice struct {
	mu sync.Mutex
	bytes.Buffer
}
