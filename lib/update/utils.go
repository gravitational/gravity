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

package update

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// WaitForEndpoints waits for service endpoints to become active for the server specified with nodeID.
// nodeID is assumed to be the name of the node as accepted by Kubernetes
func WaitForEndpoints(ctx context.Context, client corev1.CoreV1Interface, nodeID string) error {
	clusterLabels := labels.Set{"app": defaults.GravityClusterLabel}
	err := Retry(ctx, func() error {
		if hasEndpoints(client, clusterLabels, existingEndpoint) == nil {
			return nil
		}
		return trace.NotFound("endpoints not ready")
	}, defaults.EndpointsWaitTimeout)
	return trace.Wrap(err)
}

// Retry runs the specified function fn.
// If the function fails, it is retried for the given timeout using exponential backoff
func Retry(ctx context.Context, fn func() error, timeout time.Duration) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = timeout
	return trace.Wrap(utils.RetryWithInterval(ctx, b, fn))
}

func hasEndpoints(client corev1.CoreV1Interface, labels labels.Set, fn endpointMatchFn) error {
	list, err := client.Endpoints(metav1.NamespaceSystem).List(
		metav1.ListOptions{
			LabelSelector: labels.String(),
		},
	)
	if err != nil {
		log.WithError(err).Warn("Failed to query endpoints.")
		return trace.Wrap(rigging.ConvertError(err), "failed to query endpoints")
	}
	for _, endpoint := range list.Items {
		for _, subset := range endpoint.Subsets {
			for _, addr := range subset.Addresses {
				log.WithField("addr", addr).Debug("Trying endpoint.")
				if fn(addr) {
					return nil
				}
			}
		}
	}
	log.WithField("query", labels).Warn("No active endpoints found.")
	return trace.NotFound("no active endpoints found for query %q", labels)
}

// existingEndpoint is a trivial predicate that matches for any endpoint.
func existingEndpoint(v1.EndpointAddress) bool {
	return true
}

// endpointMatchFn matches an endpoint address using custom criteria.
type endpointMatchFn func(addr v1.EndpointAddress) bool
