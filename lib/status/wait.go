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

package status

import (
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// Wait blocks until the local agent reports healthy status or until the
// provided context expires.
func Wait(ctx context.Context) error {
	b := utils.NewExponentialBackOff(defaults.NodeStatusTimeout)
	return utils.RetryWithInterval(ctx, b, func() error {
		return getLocalNodeStatus(ctx)
	})
}

func getLocalNodeStatus(ctx context.Context) error {
	status, err := FromLocalPlanetAgent(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if status.GetSystemStatus() != agentpb.SystemStatus_Running {
		return trace.BadParameter("node is degraded")
	}
	return nil
}
