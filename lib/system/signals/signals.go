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

// package signals implements support for managing interrupt signals
package signals

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"
)

// WatchTerminationSignals stops the provided stopper when it gets one of monitored signals.
// It is a convenience wrapper over NewInterruptHandler
func WatchTerminationSignals(ctx context.Context, cancel context.CancelFunc, stopper Stopper, printer utils.Printer) *InterruptHandler {
	handler := NewInterruptHandler(ctx)
	handler.AddStopper(stopper)
	for {
		select {
		case sig := <-handler.C:
			printer.Println("Received", sig, "signal, terminating.")
			cancel()
		case <-ctx.Done():
		}
	}
	return handler
}

// NewInterruptHandler creates a new interrupt handler for the specified configuration.
//
// Use the select loop and handle the receives on the interrupt channel:
//
// ctx, cancel := ...
// handler := NewInterruptHandler(ctx)
// for {
// 	select {
//  	case <-handler.C:
// 		if shouldTerminate() [
// 			cancel()
// 		}
// 	case <-ctx.Done():
//		// Done
// 	}
// }
func NewInterruptHandler(ctx context.Context, opts ...InterruptOption) *InterruptHandler {
	var stoppers []Stopper
	interruptC := make(chan os.Signal)
	termC := make(chan []Stopper, 1)
	var wg sync.WaitGroup
	handler := &InterruptHandler{
		C:     interruptC,
		ctx:   ctx,
		termC: termC,
		wg:    wg,
	}
	for _, f := range opts {
		f(handler)
	}
	if len(handler.signals) == 0 {
		handler.signals = signals
	}
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, handler.signals...)
	wg.Add(1)
	go func() {
		defer func() {
			// Reset the signal handler so the next signal is handled
			// directly by the runtime
			signal.Reset(handler.signals...)
			if len(stoppers) == 0 {
				wg.Done()
				return
			}
			localCtx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
			for _, stopper := range stoppers {
				stopper.Stop(localCtx)
			}
			cancel()
			wg.Done()
		}()
		for {
			select {
			case handlers := <-termC:
				stoppers = append(stoppers, handlers...)
			case sig := <-signalC:
				select {
				case interruptC <- sig:
				case <-ctx.Done():
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return handler
}

// Close closes the loop and waits until all internal processes have stopped
func (r *InterruptHandler) Close() {
	r.wg.Wait()
}

// Done returns the channel that signals when this handler
// is closed
func (r *InterruptHandler) Done() <-chan struct{} {
	return r.ctx.Done()
}

// Add adds stoppers to the internal termination loop
func (r *InterruptHandler) AddStopper(stoppers ...Stopper) {
	select {
	case r.termC <- stoppers:
		// Added new stopper
	case <-r.ctx.Done():
	}
}

// InterruptHandler defines an interruption signal handler
type InterruptHandler struct {
	// C is the channel that receives interrupt requests
	C       <-chan os.Signal
	ctx     context.Context
	termC   chan<- []Stopper
	signals []os.Signal
	wg      sync.WaitGroup
}

// WithSignals specifies which signal to consider interrupt signals.
func WithSignals(signals ...os.Signal) InterruptOption {
	return func(h *InterruptHandler) {
		h.signals = signals
	}
}

// InterruptOption is a functional option to configure interrupt handler
type InterruptOption func(*InterruptHandler)

// Stopper is a common interface for everything that can be stopped with a context
type Stopper interface {
	// Stop performs implementation-specific cleanup tasks bound by the provided context
	Stop(context.Context) error
}

// Stop invokes this stopper function.
// Stop implements Stopper
func (r StopperFunc) Stop(ctx context.Context) error {
	return r(ctx)
}

// StopperFunc is an adapter function that allows the use
// of ordinary functions as Stoppers
type StopperFunc func(context.Context) error

// signals lists default interruption signals
var signals = []os.Signal{
	syscall.SIGINT,
	syscall.SIGTERM,
	syscall.SIGQUIT,
}
