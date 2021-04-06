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

package phases

import (
	"context"
	"io"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewApp returns executor that runs install and post-install hooks
func NewApp(p fsm.ExecutorParams, operator ops.Operator, apps app.Applications) (*hookExecutor, error) {
	return NewHook(p, operator, apps,
		schema.HookInstall,
		schema.HookInstalled,
		schema.HookStatus)
}

// NewHook returns executor that runs specified application hooks
func NewHook(p fsm.ExecutorParams, operator ops.Operator, apps app.Applications, hooks ...schema.HookType) (*hookExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.ServiceUser == nil {
		return nil, trace.BadParameter("service user is required")
	}

	serviceUser, err := systeminfo.UserFromOSUser(*p.Phase.Data.ServiceUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &hookExecutor{
		FieldLogger:    logger,
		Operator:       operator,
		Apps:           apps,
		ExecutorParams: p,
		Hooks:          hooks,
		ServiceUser:    *serviceUser,
	}, nil
}

type hookExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Operator is installer ops service
	Operator ops.Operator
	// Apps is the app service that runs the hook
	Apps app.Applications
	// ServiceUser is the user used for services and system storage
	ServiceUser systeminfo.User
	// Hooks is hook names to be executed
	Hooks []schema.HookType
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute runs install and post install hooks for an app
func (p *hookExecutor) Execute(ctx context.Context) error {
	err := p.runHooks(ctx, p.Hooks...)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// runHooks runs specified app hooks
func (p *hookExecutor) runHooks(ctx context.Context, hooks ...schema.HookType) error {
	for _, hook := range hooks {
		locator := *p.Phase.Data.Package
		req := app.HookRunRequest{
			Application: locator,
			Hook:        hook,
			Values:      p.Phase.Data.Values,
			ServiceUser: storage.OSUser{
				Name: p.ServiceUser.Name,
				UID:  strconv.Itoa(p.ServiceUser.UID),
				GID:  strconv.Itoa(p.ServiceUser.GID),
			},
		}
		if hook == schema.HookNetworkInstall {
			req.HostNetwork = true
		}

		_, err := app.CheckHasAppHook(p.Apps, req)
		if err != nil {
			if trace.IsNotFound(err) {
				p.Debugf("Application %v does not have %v hook.",
					locator, hook)
				continue
			}
			return trace.Wrap(err)
		}
		p.Progress.NextStep("Executing %v hook for %v:%v", hook,
			locator.Name, locator.Version)
		p.Infof("Executing %v hook for %v:%v.", hook, locator.Name, locator.Version)
		reader, writer := io.Pipe()
		go func() {
			defer reader.Close()
			err := p.Operator.StreamOperationLogs(p.Key(), reader)
			if err != nil && !utils.IsStreamClosedError(err) {
				logrus.Warnf("Error streaming hook logs: %v.",
					trace.DebugReport(err))
			}
		}()
		_, err = app.StreamAppHook(ctx, p.Apps, req, writer)
		if err != nil {
			return trace.Wrap(err, "%v %s hook failed", locator, hook)
		}
		// closing the writer will result in the reader returning io.EOF
		// so the goroutine above will gracefully finish streaming
		err = writer.Close()
		if err != nil {
			logrus.Warnf("Failed to close pipe writer: %v.", err)
		}
	}
	return nil
}

// Rollback is no-op for this phase
func (*hookExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*hookExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*hookExecutor) PostCheck(ctx context.Context) error {
	return nil
}
