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

package clusterconfig

import (
	"context"
	"net"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/internal/rollingupdate"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewOperationPlan creates a new operation plan for the specified operation
func NewOperationPlan(
	operator ops.Operator,
	apps app.Applications,
	client *kubernetes.Clientset,
	operation ops.SiteOperation,
	clusterConfig clusterconfig.Interface,
	servers []storage.Server,
) (plan *storage.OperationPlan, err error) {
	cluster, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := apps.GetApp(cluster.App.Package)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query installed application")
	}
	var services []v1.Service
	updatesServiceCIDR := clusterConfig.GetGlobalConfig().ServiceCIDR != ""
	if updatesServiceCIDR {
		services, err = collectServices(client.CoreV1(), clusterConfig.GetGlobalConfig().ServiceCIDR)
		if err != nil {
			return nil, trace.Wrap(err, "failed to collect existing kubernetes services")
		}
	}
	plan, err = newOperationPlan(*app, cluster.DNSConfig, operator, operation, clusterConfig, servers, services)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = operator.CreateOperationPlan(operation.Key(), *plan)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotImplemented(
				"cluster operator does not implement the API required to update cluster configuration. " +
					"Please make sure you're running the command on a compatible cluster.")
		}
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

// newOperationPlan returns a new plan for the specified operation
// and the given set of servers
func newOperationPlan(
	app app.Application,
	dnsConfig storage.DNSConfig,
	operator rollingupdate.ConfigPackageRotator,
	operation ops.SiteOperation,
	clusterConfig clusterconfig.Interface,
	servers []storage.Server,
	services []v1.Service,
) (plan *storage.OperationPlan, err error) {
	updatesServiceCIDR := clusterConfig.GetGlobalConfig().ServiceCIDR != ""
	var builder *builder
	var updates []storage.UpdateServer
	if updatesServiceCIDR {
		builder, err = newBuilderWithServices(app.Package, services, clusterConfig.GetGlobalConfig().ServiceCIDR)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updates, err = rollingupdate.RuntimeConfigUpdatesWithSecrets(app.Manifest, operator, operation.Key(), servers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		builder = newBuilder(app.Package)
		updates, err = rollingupdate.RuntimeConfigUpdates(app.Manifest, operator, operation.Key(), servers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	masters, nodes := update.SplitServers(updates)
	if len(masters) == 0 {
		return nil, trace.NotFound("no master servers found in cluster state")
	}
	shouldUpdateNodes := shouldUpdateNodes(clusterConfig, len(nodes))
	updateServers := updates
	if !shouldUpdateNodes {
		updateServers = masters
	}
	config := *builder.Config("Update runtime configuration", updateServers)
	updateMasters := *builder.Masters(
		masters,
		"Update cluster configuration",
		"Update configuration on node %q",
	).Require(config)
	phases := update.Phases{config, updateMasters}

	if shouldUpdateNodes {
		updateNodes := *builder.Nodes(
			nodes, masters[0].Server,
			"Update cluster configuration",
			"Update configuration on node %q",
		).Require(config, updateMasters)
		phases = append(phases, updateNodes)
	}
	if updatesServiceCIDR {
		init := builder.init("Init operation")
		fini := builder.fini("Finalize operation")
		phases = append([]update.Phase{init}, append(phases, fini)...)
	}

	plan = &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Phases:        phases.AsPhases(),
		Servers:       servers,
		DNSConfig:     dnsConfig,
	}
	update.ResolvePlan(plan)

	return plan, nil
}

func collectServices(client corev1.CoreV1Interface, serviceCIDR string) (result []v1.Service, err error) {
	_, ipNet, err := net.ParseCIDR(serviceCIDR)
	if err != nil {
		return nil, trace.Wrap(err, "invalid service subnet: %v", serviceCIDR)
	}
	services, err := client.Services(constants.AllNamespaces).List(metav1.ListOptions{})
	if err != nil {
		return nil, rigging.ConvertError(err)
	}
	logger := logrus.WithField(trace.Component, "clusterconfig")
	result = make([]v1.Service, 0, len(services.Items))
	for _, service := range services.Items {
		// This does not work exclusively on services of type v1.ServiceTypeClusterIP
		// as at least gravity-site service uses v1.ServiceTypeLoadBalancer even for
		// on-prem installs
		if !shouldCollectService(service) {
			continue
		}
		if ipNet.Contains(net.ParseIP(service.Spec.ClusterIP)) {
			utils.WithService(service, logger).Warn("Service not from current network range - will skip.")
			continue
		}
		result = append(result, service)
	}
	return result, nil
}

func hasServiceCIDRUpdate(clusterConfig clusterconfig.Interface) bool {
	return len(clusterConfig.GetGlobalConfig().ServiceCIDR) != 0
}

func shouldCollectService(service v1.Service) bool {
	return service.Spec.ClusterIP != "" && !utils.IsHeadlessService(service)
}

func shouldUpdateNodes(clusterConfig clusterconfig.Interface, numWorkerNodes int) bool {
	if numWorkerNodes == 0 {
		return false
	}
	var hasComponentUpdate, hasCIDRUpdate bool
	config := clusterConfig.GetGlobalConfig()
	hasComponentUpdate = len(config.FeatureGates) != 0
	hasCIDRUpdate = len(config.PodCIDR) != 0 || len(config.ServiceCIDR) != 0
	return !clusterConfig.GetKubeletConfig().IsEmpty() || hasComponentUpdate || hasCIDRUpdate
}
