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
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/sirupsen/logrus"
)

// WatchTerminationSignals stops the provided stopper when it gets one of monitored signals
func WatchTerminationSignals(ctx context.Context, cancel context.CancelFunc, stopper Stopper, log logrus.FieldLogger) {
	signalC := make(chan os.Signal, 1)
	signals := []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}
	signal.Notify(signalC, signals...)
	log.Debugf("Installed signal handler: %v.", signals)
	go func() {
		defer func() {
			localCtx, localCancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
			stopper.Stop(localCtx)
			localCancel()
			cancel()
		}()
		for {
			select {
			case <-ctx.Done():
				signal.Reset(signals...)
				return
			case sig := <-signalC:
				signal.Reset(signals...)
				fmt.Printf("Received %q signal, shutting down...\n", sig)
				log.Infof("Received %q signal.", sig)
				return
			}
		}
	}()
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
