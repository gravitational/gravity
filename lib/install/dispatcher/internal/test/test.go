/*
Copyright 2019 Gravitational, Inc.

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
package test

import (
	"time"

	"github.com/gravitational/gravity/lib/install/dispatcher"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/ops"

	. "gopkg.in/check.v1" //nolint:revive,stylecheck // TODO: tests will be rewritten to use testify
)

// ValidateResponse validates that the dispatcher contains a response
// with the given message
func ValidateResponse(c *C, dispatcher dispatcher.EventDispatcher, msg string) {
	resp := <-dispatcher.Chan()
	c.Assert(resp, DeepEquals, &installpb.ProgressResponse{
		Message: msg,
	})
}

// NewResponse creates a new response with the given message
func NewResponse(msg string) *installpb.ProgressResponse {
	return &installpb.ProgressResponse{
		Message: msg,
	}
}

// NewEvent creates a new event with the given message
func NewEvent(msg string) dispatcher.Event {
	return dispatcher.Event{
		Progress: &ops.ProgressEntry{Message: msg},
	}
}

// Timeout specifies the maximum amount of time to wait for an expectation in time-based tests
const Timeout = 500 * time.Millisecond
