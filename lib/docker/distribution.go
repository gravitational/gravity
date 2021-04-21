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

package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"runtime"

	"github.com/docker/distribution/configuration"
	registrycontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/handlers"
	"github.com/docker/distribution/registry/listener"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem"
	"github.com/docker/distribution/version"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	sysloghook "github.com/sirupsen/logrus/hooks/syslog"
)

// NewRegistry creates a new registry instance from the specified configuration.
func NewRegistry(config *configuration.Configuration) (*Registry, error) {
	ctx, cancel := defaultContext()
	app := handlers.NewApp(ctx, config)
	app.RegisterHealthChecks()
	handler := alive("/", app)

	server := &http.Server{
		Handler: handler,
	}

	return &Registry{
		app:    app,
		config: config,
		server: server,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start starts the registry server and returns when the server
// has actually started listening.
func (r *Registry) Start() error {
	listener, err := listener.NewListener(r.config.HTTP.Net, r.config.HTTP.Addr)
	if err != nil {
		return trace.Wrap(err)
	}
	r.addr = listener.Addr()
	registrycontext.GetLogger(r.app).Infof("Listening on %v.", listener.Addr())
	go func() {
		if err := r.listenAndServe(listener); err != nil {
			registrycontext.GetLogger(r.app).Warnf("Failed to serve: %v.", err)
		}
	}()
	return trace.Wrap(err)
}

// listenAndServe runs the registry's HTTP server.
// listener is closed upon exit
func (r *Registry) listenAndServe(listener net.Listener) error {
	err := r.server.Serve(listener)
	listener.Close()
	return trace.Wrap(err)
}

// Addr returns the address this registry listens on.
func (r *Registry) Addr() string {
	if runtime.GOOS == "darwin" {
		switch addr := r.addr.(type) {
		case *net.TCPAddr:
			// See https://github.com/docker/for-mac/issues/3611
			// FIXME(dima): avoid hardcoding 0.0.0.0
			return fmt.Sprintf("0.0.0.0:%d", addr.Port)
		}
	}
	return r.addr.String()
}

// Close shuts down the registry.
func (r *Registry) Close() error {
	r.cancel()
	return nil
}

// A Registry represents a complete instance of the registry.
type Registry struct {
	config *configuration.Configuration
	app    *handlers.App
	server *http.Server
	ctx    context.Context
	cancel context.CancelFunc
	addr   net.Addr
}

// alive simply wraps the handler with a route that always returns an http 200
// response when the path is matched. If the path is not matched, the request
// is passed to the provided handler. There is no guarantee of anything but
// that the server is up. Wrap with other handlers (such as health.Handler)
// for greater affect.
func alive(path string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path {
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// BasicConfiguration creates a configuration object for running
// a local registry server on the specified address addr and using rootdir
// as a root directory for a filesystem driver
func BasicConfiguration(addr, rootdir string) *configuration.Configuration {
	config := &configuration.Configuration{
		Version: "0.1",
		Storage: configuration.Storage{
			"cache":      configuration.Parameters{"blobdescriptor": "inmemory"},
			"filesystem": configuration.Parameters{"rootdirectory": rootdir},
		},
	}
	config.HTTP.Addr = addr
	config.HTTP.Headers = http.Header{
		"X-Content-Type-Options": []string{"nosniff"},
	}
	return config
}

func defaultContext() (context.Context, context.CancelFunc) {
	ctx := registrycontext.WithVersion(context.Background(), version.Version)
	ctx = registrycontext.WithLogger(ctx, newLogger())
	return context.WithCancel(ctx)
}

func newLogger() registrycontext.Logger {
	logger := log.New()
	logger.SetLevel(log.WarnLevel)
	logger.SetHooks(make(log.LevelHooks))
	hook, err := sysloghook.NewSyslogHook("", "", syslog.LOG_WARNING, "")
	if err != nil {
		logger.Out = os.Stderr
	} else {
		logger.AddHook(hook)
		logger.Out = ioutil.Discard
	}
	// distribution expects an instance of log.Entry
	return logger.WithField("source", "local-docker-registry")
}
