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

	"github.com/gravitational/gravity/lib/fsm"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewFinal returns a new instance of the last configuration step
func NewFinal(params fsm.ExecutorParams, client corev1.CoreV1Interface, logger log.FieldLogger) (*Final, error) {
	return &Final{
		FieldLogger: logger,
		client:      client,
		suffix:      serviceSuffix(params.Phase.Data.Update.ClusterConfig.ServiceSuffix),
	}, nil
}

// Execute renames the new DNS services so they persist and removes the old services
func (r *Final) Execute(ctx context.Context) error {
	services := r.client.Services(metav1.NamespaceSystem)
	for _, service := range []string{r.suffix.serviceName(), r.suffix.workerServiceName()} {
		if err := removeService(ctx, service, metav1.DeleteOptions{}, services); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Rollback is a no-op for this phase
func (r *Final) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*Final) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*Final) PostCheck(context.Context) error {
	return nil
}

// Final implements the last step for the cluster configuration update operation.
// On the happy path, it will remove the temporary DNS services.
// During rollback, it removes the active DNS services which will be recreated by the
// planet agent service after the container reverts to the previous configuration
type Final struct {
	log.FieldLogger
	client corev1.CoreV1Interface
	suffix serviceSuffix
}
