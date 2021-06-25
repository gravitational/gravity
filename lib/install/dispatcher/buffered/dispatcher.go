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

// package buffered implements an event dispatcher with buffering semantics
package buffered

import (
	"context"
	"sync"

	"github.com/gravitational/gravity/lib/install/dispatcher"
	installpb "github.com/gravitational/gravity/lib/install/proto"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New creates an event dispatcher with internal message buffer
// to collect messages that it failed to send to the client.
// The messages are resent whenever the client reconnects.
func New() *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Dispatcher{
		log:     log.WithField(trace.Component, "buffered:dispatcher"),
		ctx:     ctx,
		cancel:  cancel,
		respC:   make(chan *installpb.ProgressResponse),
		notifyC: make(chan *installpb.ProgressResponse),
	}
	d.startMessageBufferLoop()
	return d
}

// Send publishes the specified event.
// Implements EventDispatcher
func (r *Dispatcher) Send(event dispatcher.Event) {
	select {
	case r.respC <- event.AsProgressResponse():
	case <-r.ctx.Done():
	}
}

// Close stops the dispatcher internal processes
// Implements EventDispatcher
func (r *Dispatcher) Close() {
	r.cancel()
	r.wg.Wait()
}

// Chan returns the channel that receives events
func (r *Dispatcher) Chan() <-chan *installpb.ProgressResponse {
	return r.notifyC
}

// startMessageBufferLoop starts the message buffering loop for the default message
// handler to account for client dropping and reconnecting later.
func (r *Dispatcher) startMessageBufferLoop() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		var notifyC chan *installpb.ProgressResponse
		var first *installpb.ProgressResponse
		// pending accumulates the progress messages we could not send
		// to the receiver.
		// It is unbounded but the installer is not expected to have a large
		// number of progress messages so it is an acceptable compromise
		var pending []*installpb.ProgressResponse
		for {
			select {
			case resp := <-r.respC:
				pending = append(pending, resp)
				first = pending[0]
				notifyC = r.notifyC
			case notifyC <- first:
				pending = pending[1:]
				if len(pending) == 0 {
					notifyC = nil
					first = nil
				} else {
					first = pending[0]
				}
			case <-r.ctx.Done():
				if len(pending) != 0 {
					for _, resp := range pending {
						select {
						case r.notifyC <- resp:
						default:
						}
					}
				}
				close(r.notifyC)
				r.log.Info("Buffer loop done.")
				return
			}
		}
	}()
}

// Dispatcher is a buffering progress event dispatcher.
// It will buffer events if the receiving side is slow or unavailable
// until it has reconnected.
type Dispatcher struct {
	log     log.FieldLogger
	ctx     context.Context
	cancel  context.CancelFunc
	respC   chan *installpb.ProgressResponse
	notifyC chan *installpb.ProgressResponse
	wg      sync.WaitGroup
}
