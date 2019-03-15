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
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// SyncOperationPlan will synchronize the specified operation and its plan from source to destination backend
func SyncOperationPlan(src storage.Backend, dst storage.Backend, plan storage.OperationPlan, operation storage.SiteOperation) error {
	cluster, err := src.GetSite(plan.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = dst.CreateSite(*cluster)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	_, err = dst.CreateSiteOperation(operation)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	_, err = dst.CreateOperationPlan(plan)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	return trace.Wrap(SyncChangelog(src, dst, plan.ClusterName, plan.OperationID))
}

// WaitForEndpoints waits for service endpoints to become active for the server specified with nodeID.
// nodeID is assumed to be the name of the node as accepted by Kubernetes
func WaitForEndpoints(ctx context.Context, client corev1.CoreV1Interface, server storage.Server) error {
	clusterLabels := labels.Set{"app": defaults.GravityClusterLabel}
	kubednsLegacyLabels := labels.Set{"k8s-app": "kubedns"}
	kubednsLabels := labels.Set{"k8s-app": defaults.KubeDNSLabel}
	kubednsWorkerLabels := labels.Set{"k8s-app": defaults.KubeDNSWorkerLabel}

	// Due to https://github.com/gravitational/gravity.e/issues/3808 the node name we need to match may be inconsistent
	// so try to match either possible node name
	matchesNode := matchesNode([]string{
		server.AdvertiseIP,
		server.Nodename,
	})
	err := Retry(ctx, func() error {
		if (hasEndpoints(client, clusterLabels, existingEndpoint) == nil) &&
			(hasEndpoints(client, kubednsLabels, matchesNode) == nil ||
				hasEndpoints(client, kubednsLegacyLabels, matchesNode) == nil ||
				hasEndpoints(client, kubednsWorkerLabels, matchesNode) == nil) {
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

// SplitServers splits the specified server list into servers with master cluster role
// and regular nodes.
func SplitServers(servers []storage.UpdateServer) (masters, nodes []storage.UpdateServer) {
	for _, server := range servers {
		switch server.ClusterRole {
		case string(schema.ServiceRoleMaster):
			masters = append(masters, server)
		case string(schema.ServiceRoleNode):
			nodes = append(nodes, server)
		}
	}
	return masters, nodes
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

// matchesNode is a predicate that matches an endpoint address to the specified
// node name
func matchesNode(nodes []string) endpointMatchFn {
	return func(addr v1.EndpointAddress) bool {
		// Abort if the node name is not populated.
		// There is no need to wait for endpoints we cannot
		// match to a node.
		if addr.NodeName == nil {
			return false
		}

		for _, node := range nodes {
			if *addr.NodeName == node {
				return true
			}
		}
		return false
	}
}

// existingEndpoint is a trivial predicate that matches for any endpoint.
func existingEndpoint(v1.EndpointAddress) bool {
	return true
}

// endpointMatchFn matches an endpoint address using custom criteria.
type endpointMatchFn func(addr v1.EndpointAddress) bool

func formatOperation(op ops.SiteOperation) string {
	return fmt.Sprintf("operation(%v(%v), cluster=%v, created=%v)",
		op.TypeString(), op.ID, op.SiteDomain, op.Created.Format(constants.ShortDateFormat))
}
