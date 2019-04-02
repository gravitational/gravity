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

package utils

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// WatchTerminationSignals stops the provided stopper when it gets one of monitored signals
func WatchTerminationSignals(ctx context.Context, cancel context.CancelFunc, stopper Stopper, logger logrus.FieldLogger) {
	updateCh := WatchTerminationSignalsWithChannel(ctx, cancel, logger)
	select {
	case updateCh <- stopper:
	case <-ctx.Done():
	}
}

// WatchTerminationSignalsWithChannel invokes the specified cancel when it receives an interrupt
// signal.
// Returns a channel to update the list of stoppers to stop upon termination
func WatchTerminationSignalsWithChannel(ctx context.Context, cancel context.CancelFunc, logger logrus.FieldLogger) chan<- Stopper {
	signalC := make(chan os.Signal, 1)
	signals := []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}
	signal.Notify(signalC, signals...)
	var stoppers []Stopper
	updateCh := make(chan Stopper, 1)
	go func() {
		defer func() {
			if len(stoppers) == 0 {
				cancel()
				return
			}
			localCtx, localCancel := context.WithTimeout(ctx, 5*time.Second)
			for _, stopper := range stoppers {
				stopper.Stop(localCtx)
			}
			localCancel()
		}()
		for {
			select {
			case <-ctx.Done():
				signal.Reset(signals...)
				return
			case stopper := <-updateCh:
				stoppers = append(stoppers, stopper)
			case sig := <-signalC:
				signal.Reset(signals...)
				logger.WithField("signal", sig).Info("Received signal, shutting down...")
				return
			}
		}
	}()
	return updateCh
}

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
