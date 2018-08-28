package utils

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// WatchTerminationSignals stops the provided stopper when it gets one of monitored signals
func WatchTerminationSignals(ctx context.Context, cancel context.CancelFunc, stopper Stopper, log logrus.FieldLogger) {
	signalC := make(chan os.Signal, 1)
	signals := []os.Signal{
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}
	signal.Notify(signalC, signals...)
	log.Debugf("Installed signal handler: %v.", signals)
	go func() {
		defer func() {
			localCtx, localCancel := context.WithTimeout(ctx, 5*time.Second)
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
