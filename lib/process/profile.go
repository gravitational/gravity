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

package process

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	//nolint:gosec // imported for side-effects
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sync/atomic"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

var profilingStarted int32

// StartProfiling starts profiling endpoint, will return AlreadyExists
// if profiling has been initiated
func StartProfiling(ctx context.Context, httpEndpoint, profileDir string) error {
	if !atomic.CompareAndSwapInt32(&profilingStarted, 0, 1) {
		return trace.AlreadyExists("profiling has been already started")
	}

	listener, err := net.Listen("tcp", httpEndpoint)
	if err != nil {
		return trace.Wrap(err, "failed to start profiling on %v", httpEndpoint)
	}

	logger := log.WithFields(log.Fields{
		trace.Component: "profiling",
		"pid":           os.Getpid(),
		"addr":          listener.Addr(),
		"cmdline":       os.Args,
		"curl":          fmt.Sprintf("%v/debug/pprof/goroutine?debug=1", listener.Addr()),
	})
	if profileDir != "" {
		log.WithField("profile-dir", profileDir).Info("Started.")
	} else {
		logger.Info("Started.")
	}

	go func() {
		logger.Println(http.Serve(listener, nil))
	}()

	if profileDir == "" {
		return nil
	}

	profileDir = filepath.Join(profileDir, fmt.Sprintf("%v", os.Getpid()))
	if err := os.MkdirAll(profileDir, defaults.SharedDirMask); err != nil {
		return trace.Wrap(err, "failed to create directory %v", profileDir)
	}

	logger.Info("Setting up periodic profile dumps.")
	go func() {
		ticker := time.NewTicker(defaults.ProfilingInterval)
		for {
			select {
			case <-ticker.C:
				f, err := ioutil.TempFile(profileDir, "stacks")
				if err == nil {
					err = pprof.Lookup("goroutine").WriteTo(f, 1)
					if err != nil {
						logger.WithError(err).Warn("Failed to dump goroutine profile.")
					}
					f.Close()
				}
				f, err = ioutil.TempFile(profileDir, "heap")
				if err == nil {
					err = pprof.WriteHeapProfile(f)
					if err != nil {
						logger.WithError(err).Warn("Failed to dump heap profile.")
					}
					f.Close()
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}
