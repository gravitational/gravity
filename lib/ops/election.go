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

package ops

import (
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// EnableLeaderElection turns on leader election for the specified node.
func EnableLeaderElection(ctx context.Context, clusterName string, node storage.Server, log logrus.FieldLogger) error {
	return runLeaderCommandRetry(ctx, "resume", clusterName, node, log)
}

// PauseLeaderElection pauses leader election for the specified node.
func PauseLeaderElection(ctx context.Context, clusterName string, node storage.Server, log logrus.FieldLogger) error {
	return runLeaderCommandRetry(ctx, "pause", clusterName, node, log)
}

func runLeaderCommandRetry(ctx context.Context, command string, clusterName string, node storage.Server, log logrus.FieldLogger) error {
	b := backoff.NewExponentialBackOff()
	b.MaxInterval = defaults.ElectionRetryMaxInterval
	b.MaxElapsedTime = defaults.ElectionWaitTimeout
	return utils.RetryTransient(ctx, b, func() error {
		return runLeaderCommand(ctx, command, clusterName, node, log)
	})
}

func runLeaderCommand(ctx context.Context, command string, clusterName string, node storage.Server, log logrus.FieldLogger) error {
	out, err := utils.RunPlanetCommand(ctx, log, "leader", command,
		fmt.Sprintf("--public-ip=%v", node.AdvertiseIP),
		fmt.Sprintf("--election-key=/planet/cluster/%v/election", clusterName),
		"--etcd-cafile=/var/state/root.cert",
		"--etcd-certfile=/var/state/etcd.cert",
		"--etcd-keyfile=/var/state/etcd.key")
	if err != nil {
		return trace.Wrap(err, "failed to enable election for %v: %s",
			node.AdvertiseIP, string(out))
	}
	return nil
}
