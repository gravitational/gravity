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

// package direct implements a synchronous event dispatcher
package direct

import (
	"context"
	"sync"

	"github.com/gravitational/gravity/lib/install/dispatcher"
	installpb "github.com/gravitational/gravity/lib/install/proto"
)

// New creates a new event dispatcher
func New() *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Dispatcher{
		ctx:     ctx,
		cancel:  cancel,
		respC:   make(chan *installpb.ProgressResponse),
		notifyC: make(chan *installpb.ProgressResponse),
	}
	d.startMessageLoop()
	return d
}

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

// Chan returns the channel that receives progress updates
func (r *Dispatcher) Chan() <-chan *installpb.ProgressResponse {
	return r.notifyC
}

func (r *Dispatcher) startMessageLoop() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			select {
			case resp := <-r.respC:
				select {
				case r.notifyC <- resp:
				case <-r.ctx.Done():
				}
			case <-r.ctx.Done():
				close(r.notifyC)
				return
			}
		}
	}()
}

// Dispatcher implements event dispatcher that dispatches events
// without intermediate buffering.
// It requires that the receiving side is servicing the notification channel
// otherwise it will block
type Dispatcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	respC   chan *installpb.ProgressResponse
	notifyC chan *installpb.ProgressResponse
	wg      sync.WaitGroup
}
