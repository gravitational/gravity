/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/gravitational/gravity/lib/defaults"
	libstatus "github.com/gravitational/gravity/lib/status"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type checkLocalHealth struct {
	log.FieldLogger
}

// NewPhaseNodeHealth creates an upgrade phase to check whether the node is healthy
func NewPhaseNodeHealth(logger log.FieldLogger) (*checkLocalHealth, error) {
	return &checkLocalHealth{
		FieldLogger: logger,
	}, nil
}

// Execute will block progress until the node enters a healthy state
func (p *checkLocalHealth) Execute(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.NodeStatusTimeout)
	defer cancel()

	return trace.Wrap(libstatus.WaitForNodeHealthy(ctx))
}

// Rollback is no-op for this phase
func (p *checkLocalHealth) Rollback(context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (p *checkLocalHealth) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (p *checkLocalHealth) PostCheck(context.Context) error {
	return nil
}
