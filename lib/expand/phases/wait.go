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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	kubeutils "github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// NewWaitPlanet returns executor that waits for planet to start on the joining node
func NewWaitPlanet(p fsm.ExecutorParams, operator ops.Operator) (*waitPlanetExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &waitPlanetExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
	}, nil
}

type waitPlanetExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the wait phase
func (p *waitPlanetExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Waiting for the planet to start")
	p.Info("Waiting for the planet to start.")
	ctx, cancel := defaults.WithTimeout(ctx)
	defer cancel()
	b := backoff.NewConstantBackOff(defaults.RetryInterval)
	err := utils.RetryWithInterval(ctx, b,
		func() error {
			planetStatus, err := status.FromPlanetAgent(ctx, nil)
			if err != nil {
				return trace.Wrap(err)
			}
			for _, nodeStatus := range planetStatus.Nodes {
				if p.Phase.Data.Server.AdvertiseIP == nodeStatus.AdvertiseIP {
					if nodeStatus.Status == status.NodeHealthy {
						return nil
					}
				}
			}
			return trace.BadParameter("planet is not running yet: %#v",
				planetStatus)
		})
	if err != nil {
		return trace.Wrap(err)
	}
	p.Info("Planet is running.")
	return nil
}

// Rollback is no-op for this phase
func (*waitPlanetExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*waitPlanetExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*waitPlanetExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// NewWaitK8s returns executor that waits for Kubernetes node to register
func NewWaitK8s(p fsm.ExecutorParams, operator ops.Operator) (*waitK8sExecutor, error) {
	client, _, err := httplib.GetUnprivilegedKubeClient(p.Plan.DNSConfig.Addr())
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
	return &waitK8sExecutor{
		FieldLogger:    logger,
		Client:         client,
		ExecutorParams: p,
	}, nil
}

type waitK8sExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Client is Kubernetes client
	Client *kubernetes.Clientset
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the wait phase
func (p *waitK8sExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Waiting for the Kubernetes node to register")
	p.Info("Waiting for the Kubernetes node to register.")
	err := utils.Retry(defaults.RetryInterval, defaults.RetryAttempts,
		func() error {
			_, err := kubeutils.GetNode(p.Client, *p.Phase.Data.Server)
			return trace.Wrap(err)
		})
	if err != nil {
		return trace.Wrap(err)
	}
	p.Info("Node has registered with Kubernetes cluster.")
	return nil
}

// Rollback is no-op for this phase
func (*waitK8sExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*waitK8sExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*waitK8sExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// NewWaitTeleport returns executor that waits for Teleport node to register
func NewWaitTeleport(p fsm.ExecutorParams, operator ops.Operator) (*waitTeleportExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &waitTeleportExecutor{
		FieldLogger:    logger,
		Operator:       operator,
		ExecutorParams: p,
	}, nil
}

type waitTeleportExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Operator is the cluster operator service
	Operator ops.Operator
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute blocks until Teleport node has registered with the auth server
func (p *waitTeleportExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Waiting for the Teleport node to join the cluster")
	p.Info("Waiting for the Teleport node to join the cluster.")
	return utils.RetryFor(ctx, 2*time.Minute, func() error {
		nodes, err := p.Operator.GetClusterNodes(p.Key().SiteKey())
		if err != nil {
			return trace.Wrap(err)
		}
		for _, node := range nodes {
			if node.AdvertiseIP == p.Phase.Data.Server.AdvertiseIP {
				p.WithField("node", node).Info("Teleport node has registered.")
				return nil
			}
		}
		return trace.NotFound("Teleport on %s hasn't registered yet",
			p.Phase.Data.Server)
	})
}

// Rollback is no-op for this phase
func (*waitTeleportExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*waitTeleportExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*waitTeleportExecutor) PostCheck(ctx context.Context) error {
	return nil
}
