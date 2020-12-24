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

// package interactive implements wizard-based installation workflow
package interactive

import (
	"context"
	"errors"
	"fmt"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/dispatcher"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns a new installer that implements interactive installation
// workflow
func New(config Config) (*Engine, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Engine{
		Config: config,
	}, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.Operator == nil {
		return trace.BadParameter("Operator is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField("mode", "cli")
	}
	return nil
}

// Config defines the installer configuration
type Config struct {
	// FieldLogger is the logger for the installer
	log.FieldLogger
	// Operator specifies the service operator
	ops.Operator
	// AdvertiseAddr specifies the advertise address of the wizard
	AdvertiseAddr string
}

// Execute runs the wizard operation
func (r *Engine) Execute(ctx context.Context, installer install.Interface, config install.Config) (dispatcher.Status, error) {
	err := r.execute(ctx, installer, config)
	if err != nil {
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	return dispatcher.StatusCompletedPending, nil
}

func (r *Engine) execute(ctx context.Context, installer install.Interface, config install.Config) error {
	e, err := newExecutor(ctx, r, installer, config)
	if err != nil {
		return trace.Wrap(err)
	}

	if e.config.App.Manifest.OpsCenterDisabled() {
		message := "WebUI installs have been disabled in this application. CLI installation " +
			"docs are available at https://goteleport.com/gravity/docs/installation/#cli-installation"
		e.PrintStep(message)
		return trace.Wrap(errors.New(message))
	}

	e.printURL()
	installer.PrintStep("Waiting for the operation to start")
	operation, err := e.waitForOperation()
	if err != nil {
		return trace.Wrap(err, "failed to wait for operation to become ready")
	}
	if err := installer.NotifyOperationAvailable(*operation); err != nil {
		return trace.Wrap(err)
	}
	if err := installer.ExecuteOperation(operation.Key()); err != nil {
		return trace.Wrap(err)
	}
	if err := e.completeOperation(*operation); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func newExecutor(ctx context.Context, r *Engine, installer install.Interface, config install.Config) (*executor, error) {
	return &executor{
		Config:    r.Config,
		Interface: installer,
		ctx:       ctx,
		config:    config,
	}, nil
}

func (r *executor) waitForOperation() (operation *ops.SiteOperation, err error) {
	b := utils.NewUnlimitedExponentialBackOff()
	err = utils.RetryWithInterval(r.ctx, b, func() error {
		clusters, err := r.Operator.GetSites(defaults.SystemAccountID)
		if err != nil {
			return trace.Wrap(err, "failed to fetch clusters")
		}
		if len(clusters) == 0 {
			return trace.NotFound("no clusters created yet")
		}
		cluster := clusters[0]
		operations, err := r.Operator.GetSiteOperations(cluster.Key(), ops.OperationsFilter{})
		if err != nil {
			return trace.Wrap(err, "failed to fetch operations")
		}
		if len(operations) == 0 {
			return trace.NotFound("no operations created yet")
		}
		operation = (*ops.SiteOperation)(&operations[0])
		r.WithField("operation", operation.Key()).Info("Fetched operation.")
		if operation.State != ops.OperationStateReady {
			return trace.BadParameter("operation is not ready")
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operation, nil
}

func (r *executor) completeOperation(operation ops.SiteOperation) error {
	// With an interactive installation, the link to remote Ops Center cannot be removed
	// immediately as it is used to tunnel final install step
	if err := r.CompleteFinalInstallStep(operation.Key(), defaults.WizardLinkTTL); err != nil {
		r.WithError(err).Warn("Failed to complete final install step.")
	}
	if err := r.CompleteOperation(operation); err != nil {
		r.WithError(err).Warn("Failed to complete install.")
	}
	return nil
}

// printURL prints the URL that installer can be reached at via browser
// in interactive mode to stdout
func (r *executor) printURL() {
	r.PrintStep("Starting web UI install wizard")
	url := fmt.Sprintf("%v/web/installer/new/%v/%v/%v?install_token=%v",
		r.AdvertiseAddr,
		r.config.App.Package.Repository,
		r.config.App.Package.Name,
		r.config.App.Package.Version,
		r.config.Token.Token)
	r.WithField("installer-url", url).Info("Generated installer URL.")
	r.PrintStep(color.GreenString("Open this URL in browser: %s", url))
}

// Engine implements interactive installation workflow
type Engine struct {
	// Config specifies the engine's configuration
	Config
}

type executor struct {
	Config
	install.Interface
	config install.Config
	ctx    context.Context
}
