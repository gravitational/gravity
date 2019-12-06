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

package phases

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// registerNodeExecutor is the phase which registers a node object with the cluster
type registerNodeExecutor struct {
	// FieldLogger specifies the logger used by the executor
	log.FieldLogger
	// ExecutorParams contains common executor parameters
	fsm.ExecutorParams
	// Client is the Kubernetes client
	Client *kubernetes.Clientset
	// Taints to apply to the node based on the profile
	Taints []v1.Taint
	// Labels to apply to the node
	Labels map[string]string
}

// NewRegisterNodePhase creates a new registerNodes phase executor
func NewRegisterNodePhase(p fsm.ExecutorParams, operator ops.Operator, apps app.Applications, client *kubernetes.Clientset) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: log.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         opKey(p.Plan),
		Operator:    operator,
	}

	logrus.Info("NewRegisterNodePhase AppLocator: ", p.Phase.Data.Package)
	application, err := apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	profile, err := application.Manifest.NodeProfiles.ByName(p.Phase.Data.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &registerNodeExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Client:         client,
		Taints:         profile.Taints,
		Labels:         p.Phase.Data.Server.GetNodeLabels(profile.Labels),
	}, nil
}

// PreCheck is no-op for this phase
func (r *registerNodeExecutor) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (r *registerNodeExecutor) PostCheck(context.Context) error {
	return nil
}

// Execute generates coredns configuration
func (r *registerNodeExecutor) Execute(ctx context.Context) error {
	r.Progress.NextStep("Configuring Node")
	r.Info("Configuring Node.")

	err := utils.RetryFor(ctx, 10*time.Second, func() error {
		_, err := r.Client.CoreV1().Nodes().Create(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   r.ExecutorParams.Phase.Data.Server.KubeNodeID(),
				Labels: r.Labels,
				Annotations: map[string]string{
					// This annotation indicates that the node is managed
					// by the attach-detach controller that's running in
					// the controller manager. Without it, persistent volumes
					// won't be properly attached to the node.
					constants.AttachDetachAnnotation: "true",
				},
			},
			Spec: v1.NodeSpec{
				Taints: r.Taints,
			},
		})
		if err != nil {
			return rigging.ConvertError(err)
		}
		return nil
	})

	return trace.Wrap(err)
}

// Rollback is a noop for this executor
func (r *registerNodeExecutor) Rollback(context.Context) error {
	return nil
}
