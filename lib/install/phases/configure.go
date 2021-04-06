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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewConfigure returns a new "configure" phase executor
func NewConfigure(p fsm.ExecutorParams, operator ops.Operator) (*configureExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
	}
	var env map[string]string
	var config []byte
	if p.Phase.Data != nil && p.Phase.Data.Install != nil {
		env = p.Phase.Data.Install.Env
		config = p.Phase.Data.Install.Config
	}
	return &configureExecutor{
		FieldLogger:    logger,
		Operator:       operator,
		ExecutorParams: p,
		env:            env,
		config:         config,
	}, nil
}

type configureExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Operator is the installer process ops service
	Operator ops.Operator
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	env    map[string]string
	config []byte
}

// Execute executes the configure phase
func (p *configureExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Configuring cluster packages")
	p.Info("Configuring cluster packages.")
	err := p.Operator.ConfigurePackages(ops.ConfigurePackagesRequest{
		SiteOperationKey: fsm.OperationKey(p.Plan),
		Env:              p.env,
		Config:           p.config,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is no-op for this phase
func (p *configureExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*configureExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*configureExecutor) PostCheck(ctx context.Context) error {
	return nil
}
