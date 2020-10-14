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

package localenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/webpack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// RemoteEnvironment provides access to a remote Ops Center services
type RemoteEnvironment struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Packages is authenticated pack client
	Packages pack.PackageService
	// Apps is authenticated apps client
	Apps app.Applications
	// Operator is authenticated ops client
	Operator *opsclient.Client
	// StateDir is where this environment keeps login entries
	StateDir string
}

// NewRemoteEnvironment creates a new remote environment
func NewRemoteEnvironment() (*RemoteEnvironment, error) {
	stateDir := state.GravityInstallDir("wizard")
	err := os.MkdirAll(stateDir, defaults.SharedDirMask)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	env, err := newRemoteEnvironment(stateDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return env, nil
}

// LoginWizard creates remote environment and logs into it as a wizard user
func LoginWizard(addr, token string) (*RemoteEnvironment, error) {
	env, err := NewRemoteEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = env.LoginWizard(addr, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return env, nil
}

// LoginRemote creates remote environment and logs into with provided creds
func LoginRemote(url, token string) (*RemoteEnvironment, error) {
	env, err := NewRemoteEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = env.Login(url, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return env, nil
}

// newRemoteEnvironment creates a new remote environment that keeps its
// state in the specified dir
func newRemoteEnvironment(stateDir string) (*RemoteEnvironment, error) {
	env := &RemoteEnvironment{
		FieldLogger: logrus.WithField(trace.Component, "remoteenv"),
		StateDir:    stateDir,
	}
	// if there is a login entry, log in right away, otherwise the caller
	// will need to call Login/LoginWizard before this env can be used
	entry, err := env.wizardEntry()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return env, nil
	}
	err = env.init(*entry)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return env, nil
}

// Login logs this environment into the Ops Center with specified credentials
func (w *RemoteEnvironment) Login(url, token string) error {
	w.Debugf("Logging into Gravity Hub: %v.", url)
	_, err := w.login(storage.LoginEntry{
		Password:     token,
		OpsCenterURL: url,
	})
	return trace.Wrap(err)
}

// LoginCluster logs this environment into the specified cluster
func (w *RemoteEnvironment) LoginCluster(url, token string) error {
	w.Debugf("Logging into cluster: %v.", url)
	_, err := w.login(storage.LoginEntry{
		Password:     token,
		OpsCenterURL: url,
	})
	return trace.Wrap(err)
}

// LoginWizard logs this environment into wizard with specified address
func (w *RemoteEnvironment) LoginWizard(addr, token string) (entry *storage.LoginEntry, err error) {
	wizardPort := strconv.Itoa(defaults.WizardPackServerPort)
	var host, port string
	if strings.HasPrefix(addr, "https") {
		host, port, err = utils.URLSplitHostPort(addr, wizardPort)
		if err != nil {
			return nil, trace.Wrap(err, "invalid Gravity Hub URL %q, expected [https://]host[:port]", addr)
		}
	} else {
		host, port = utils.SplitHostPort(addr, wizardPort)
	}
	url := fmt.Sprintf("https://%v:%v", host, port)
	w.Debugf("Logging into wizard: %v.", url)
	err = w.clearWizardEntry()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return w.login(storage.LoginEntry{
		Email:        defaults.WizardUser,
		Password:     token,
		OpsCenterURL: url,
	})
}

// WaitForOperator blocks until the configured operator becomes available or context expires.
func (w *RemoteEnvironment) WaitForOperator(ctx context.Context) error {
	err := utils.RetryFor(ctx, time.Minute, func() error {
		if err := w.Operator.Ping(ctx); err != nil {
			w.Infof("Operator isn't available yet: %v.", err)
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	w.Info("Operator is available.")
	return nil
}

func (w *RemoteEnvironment) login(entry storage.LoginEntry) (*storage.LoginEntry, error) {
	err := w.withBackend(func(b storage.Backend) error {
		_, err := b.UpsertLoginEntry(entry)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = w.init(entry)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &entry, nil
}

// clearWizardEntry finds a wizard login entry and removes it
func (w *RemoteEnvironment) clearWizardEntry() error {
	entry, err := w.wizardEntry()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return nil
	}
	err = w.withBackend(func(b storage.Backend) error {
		return trace.Wrap(b.DeleteLoginEntry(entry.OpsCenterURL))
	})
	if err != nil {
		return trace.Wrap(err)
	}
	w.Debugf("Cleared old wizard entry: %v.", entry)
	return nil
}

func (w *RemoteEnvironment) init(entry storage.LoginEntry) error {
	var err error
	httpClient := httplib.GetClient(true)
	if entry.Email != "" {
		w.Packages, err = webpack.NewAuthenticatedClient(
			entry.OpsCenterURL, entry.Email, entry.Password, roundtrip.HTTPClient(httpClient))
	} else {
		w.Packages, err = webpack.NewBearerClient(
			entry.OpsCenterURL, entry.Password, roundtrip.HTTPClient(httpClient))
	}
	if err != nil {
		return trace.Wrap(err)
	}
	if entry.Email != "" {
		w.Apps, err = client.NewAuthenticatedClient(
			entry.OpsCenterURL, entry.Email, entry.Password, client.HTTPClient(httpClient))
	} else {
		w.Apps, err = client.NewBearerClient(
			entry.OpsCenterURL, entry.Password, client.HTTPClient(httpClient))
	}
	if err != nil {
		return trace.Wrap(err)
	}
	if entry.Email != "" {
		w.Operator, err = opsclient.NewAuthenticatedClient(
			entry.OpsCenterURL, entry.Email, entry.Password, opsclient.HTTPClient(httpClient))
	} else {
		w.Operator, err = opsclient.NewBearerClient(
			entry.OpsCenterURL, entry.Password, opsclient.HTTPClient(httpClient))
	}
	if err != nil {
		return trace.Wrap(err)
	}
	w.Debugf("Initialized remote environment: %s.", entry)
	return nil
}

// wizardEntry returns a login entry representing an installer process
func (w *RemoteEnvironment) wizardEntry() (*storage.LoginEntry, error) {
	var found *storage.LoginEntry
	err := w.withBackend(func(b storage.Backend) error {
		entries, err := b.GetLoginEntries()
		if err != nil {
			return trace.Wrap(err)
		}
		for _, entry := range entries {
			if entry.Email == defaults.WizardUser {
				found = &entry
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if found == nil {
		return nil, trace.NotFound("wizard login entry not found")
	}
	w.Debugf("Found wizard login entry: %s.", found)
	return found, nil
}

// withBackend executes the provided method passing it the backend
// where wizard credentials are stored and making sure backend is
// closed afterwards
func (w *RemoteEnvironment) withBackend(fn func(storage.Backend) error) (err error) {
	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path: filepath.Join(w.StateDir, "wizard.db"),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer backend.Close()
	err = fn(backend)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
