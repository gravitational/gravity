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

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewPods returns executor that removes old Kubernetes pods.
//
// All Kubernetes pods have to be recreated when the cluster comes up after
// reconfiguration to ensure that no old pods are lingering (Kubernetes may
// get confused after the old node is gone and keep pods in terminating state
// for a long time) and that they mount proper service tokens.
func NewPods(p fsm.ExecutorParams, operator ops.Operator) (*podsExecutor, error) {
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
	return &podsExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Client:         client,
	}, nil
}

type podsExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
	// Client is the Kubernetes client.
	Client *kubernetes.Clientset
}

// Execute removes the old Kubernetes pods.
func (p *podsExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Recreating Kubernetes pods")
	pods, err := p.Client.CoreV1().Pods(constants.AllNamespaces).List(ctx, metav1.ListOptions{})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, pod := range pods.Items {
		err := p.Client.CoreV1().Pods(pod.Namespace).
			Delete(ctx, pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: utils.Int64Ptr(0),
			})
		if err != nil {
			p.Errorf("Failed to remove pod %v/%v: %v.", pod.Namespace, pod.Name, err)
		} else {
			p.Infof("Removed pod %v/%v.", pod.Namespace, pod.Name)
		}
	}
	return nil
}

// Rollback is no-op for this phase.
func (*podsExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*podsExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*podsExecutor) PostCheck(ctx context.Context) error {
	return nil
}
