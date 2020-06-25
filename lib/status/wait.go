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
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "status")

// WaitCluster blocks until the local cluster is healthy or until the context
// expires.
func WaitCluster(ctx context.Context, operator ops.Operator) error {
	b := utils.NewExponentialBackOff(defaults.NodeStatusTimeout)
	return utils.RetryWithInterval(ctx, b, func() error {
		cluster, err := operator.GetLocalSite()
		if err != nil {
			return trace.Wrap(err)
		}
		status, err := FromCluster(ctx, operator, *cluster, "")
		if err != nil {
			return trace.Wrap(err)
		}
		if status.IsDegraded() {
			return trace.BadParameter("cluster is not healthy: %s", status)
		}
		log.Info("Cluster is healthy.")
		return nil
	})
}
