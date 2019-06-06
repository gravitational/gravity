package simple

import (
	"context"
	"sync"

	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/install/server/dispatcher"
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

// Chan returns the channel that receives events
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

// Dispatcher implements event dispatcher with direct event relay semantics
type Dispatcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	respC   chan *installpb.ProgressResponse
	notifyC chan *installpb.ProgressResponse
	wg      sync.WaitGroup
}
