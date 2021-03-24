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

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/gravitational/trace"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PublishDiscovery periodically updates discovery information
func (a *Autoscaler) PublishDiscovery(ctx context.Context, operator ops.Operator) {
	a.Info("Start publishing discovery info.")
	err := a.syncDiscovery(ctx, operator, true)
	if err != nil {
		a.WithError(err).Error("Failed to publish discovery.")
	}
	publishTicker := time.NewTicker(defaults.DiscoveryPublishInterval)
	resyncTicker := time.NewTicker(defaults.DiscoveryResyncInterval)
	sourceDestTicker := time.NewTicker(defaults.SourceDestinationCheckInterval)
	for {
		select {
		case <-ctx.Done():
			a.Info("Stop publishing discovery info.")
			return
		case <-publishTicker.C:
			err = a.syncDiscovery(ctx, operator, false)
			if err != nil {
				a.WithError(err).Error("Failed to publish discovery.")
			}
		case <-resyncTicker.C:
			err = a.syncDiscovery(ctx, operator, true)
			if err != nil {
				a.WithError(err).Error("Failed to publish discovery.")
			}
		case <-sourceDestTicker.C:
			err = a.checkSourceDestination(ctx, operator)
			if err != nil {
				a.WithError(err).Error("Failed to ensure source/dest check is disabled.")
			}
		}
	}
}

// checkSourceDestination makes sure source/destination check is disabled for
// the cluster instances.
func (a *Autoscaler) checkSourceDestination(ctx context.Context, operator ops.Operator) error {
	cluster, err := operator.GetLocalSite(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	var instanceIDs []string
	for _, node := range cluster.ClusterState.Servers {
		if node.InstanceID != "" {
			instanceIDs = append(instanceIDs, node.InstanceID)
		} else {
			a.Warnf("%v doesn't have cloud instance id.", node)
		}
	}
	// We don't know how EC2 API will react when provided with a large number
	// of instance IDs at once e.g. in 1K+ node clusters, so split them in
	// several smaller batches.
	for _, batch := range utils.SplitSlice(instanceIDs, 250) {
		instances, err := a.DescribeInstancesWithSourceDestinationCheck(ctx, batch)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, instance := range instances {
			a.Infof("Instance %v has source/dest check enabled, disabling.", aws.StringValue(instance.InstanceId))
			err := a.TurnOffSourceDestinationCheck(ctx, aws.StringValue(instance.InstanceId))
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// syncDiscovery syncs cluster discovery information in the SSM
func (a *Autoscaler) syncDiscovery(ctx context.Context, operator ops.Operator, force bool) error {
	cluster, err := operator.GetLocalSite(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.syncToken(ctx, operator, cluster, force); err != nil {
		return trace.Wrap(err)
	}

	if err := a.syncMasterService(ctx, force); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *Autoscaler) syncToken(ctx context.Context, operator ops.Operator, cluster *ops.Site, force bool) error {
	joinToken, err := operator.GetExpandToken(cluster.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.publishJoinToken(ctx, joinToken.Token, force); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *Autoscaler) getServiceURL() (string, error) {
	service, err := a.Client.CoreV1().Services(constants.KubeSystemNamespace).
		Get(context.TODO(), constants.GravityServiceName, v1.GetOptions{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	var port int32
	for _, p := range service.Spec.Ports {
		if p.Name == constants.GravityServicePortName {
			port = p.Port
			break
		}
	}
	if port == 0 {
		return "", trace.NotFound("no port %q found for service %q", constants.GravityServicePortName, constants.GravityServiceName)
	}
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		if ingress.Hostname != "" {
			return fmt.Sprintf("https://%v:%v", ingress.Hostname, port), nil
		}
	}
	return "", trace.NotFound("ingress load balancer not found for %v", constants.GravityServiceName)
}

func (a *Autoscaler) syncMasterService(ctx context.Context, force bool) error {
	serviceURL, err := a.getServiceURL()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.publishServiceURL(ctx, serviceURL, force)
}
