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
package buffered

import (
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/install/dispatcher"
	"github.com/gravitational/gravity/lib/install/dispatcher/internal/test"
	installpb "github.com/gravitational/gravity/lib/install/proto"

	. "gopkg.in/check.v1"
)

func TestDispatcher(t *testing.T) { TestingT(t) }

type S struct {
}

var _ = Suite(&S{})

func (S) TestBuffersEventsWhenReceiverIsUnavilable(c *C) {
	d := New()
	defer d.Close()

	// Dispatch a few events without receiver
	d.Send(test.NewEvent("first"))
	d.Send(test.NewEvent("second"))

	test.ValidateResponse(c, d, "first")
	test.ValidateResponse(c, d, "second")
}

func (S) TestDispatcherOperation(c *C) {
	d := New()
	doneC := make(chan struct{})
	var resps []*installpb.ProgressResponse
	var events = []dispatcher.Event{
		test.NewEvent("first"),
		test.NewEvent("second"),
	}

	go func(numResponses int) {
		for i := 0; i < numResponses; i++ {
			resps = append(resps, <-d.Chan())
		}
		d.Close()
		close(doneC)
	}(len(events))

	for _, event := range events {
		d.Send(event)
	}

	select {
	case <-time.After(test.Timeout):
		c.Error("timeout")
	case <-doneC:
		c.Assert(resps, DeepEquals, []*installpb.ProgressResponse{
			test.NewResponse("first"),
			test.NewResponse("second"),
		})
	}
}
