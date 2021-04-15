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
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewTokens returns executor that removes old service account tokens.
//
// During the reconfigure operation, the secrets get regenerated thus
// invalidating old service account tokens. Kubernetes will recreate
// them automatically when they are deleted during this phase.
func NewTokens(p fsm.ExecutorParams, operator ops.Operator) (*tokensExecutor, error) {
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
	return &tokensExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Client:         client,
	}, nil
}

type tokensExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
	// Client is the Kubernetes client.
	Client *kubernetes.Clientset
}

// Execute removes old service account tokens.
func (p *tokensExecutor) Execute(ctx context.Context) error {
	// Remove service account tokens.
	p.Progress.NextStep("Cleaning up Kubernetes service account tokens")
	secrets, err := p.Client.CoreV1().Secrets(constants.AllNamespaces).List(ctx, metav1.ListOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}
	for _, secret := range secrets.Items {
		// Only remove service account tokens.
		if secret.Type != v1.SecretTypeServiceAccountToken {
			p.Infof("Skipping secret %v/%v", secret.Namespace, secret.Name)
			continue
		}
		// Do not remove tokens for system controllers, Kubernetes will refresh those on its own.
		if secret.Namespace == metav1.NamespaceSystem && strings.Contains(secret.Name, "controller") {
			p.Infof("Skipping secret %v/%v", secret.Namespace, secret.Name)
			continue
		}
		err := p.Client.CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
		if err != nil {
			return trace.Wrap(err, "failed to remove secret %v/%v: %v", secret.Namespace, secret.Name, err)
		} else {
			p.Infof("Removed secret %v/%v", secret.Namespace, secret.Name)
		}
	}
	return nil
}

// Rollback is no-op for this phase.
func (*tokensExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*tokensExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*tokensExecutor) PostCheck(ctx context.Context) error {
	return nil
}
