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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewNode returns executor that removes old Kubernetes node object.
func NewNode(p fsm.ExecutorParams, operator ops.Operator) (*nodeExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	client, _, err := httplib.GetClusterKubeClient(p.Plan.DNSConfig.Addr())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &nodeExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Client:         client,
	}, nil
}

type nodeExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
	// Client is the Kubernetes client.
	Client *kubernetes.Clientset
}

// Execute removes the old Kubernetes node.
func (p *nodeExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Cleaning up Kubernetes node")
	nodes, err := utils.GetNodes(p.Client.CoreV1().Nodes())
	if err != nil {
		return trace.Wrap(err)
	}
	for ip, node := range nodes {
		if ip != p.Phase.Data.Server.AdvertiseIP {
			err := p.Client.CoreV1().Nodes().Delete(ctx, node.Name, metav1.DeleteOptions{})
			if err != nil {
				return rigging.ConvertError(err)
			}
			p.Infof("Removed Kubernetes node %v", node.Name)
		}
	}
	return nil
}

// Rollback is no-op for this phase.
func (*nodeExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*nodeExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*nodeExecutor) PostCheck(ctx context.Context) error {
	return nil
}
