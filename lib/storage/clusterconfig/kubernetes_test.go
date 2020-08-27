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

package clusterconfig

import (
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
)

type KubernetesSuite struct{}

var _ = Suite(&KubernetesSuite{})

// TestReconcile verifies the controller service can be properly reconciled to
// the desired state.
func (r *KubernetesSuite) TestReconcile(c *C) {
	testCases := []struct {
		existingServiceConfig *GravityControllerService
		updatedServiceConfig  *GravityControllerService
		expectedServiceConfig *GravityControllerService
		comment               string
	}{
		{
			expectedServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					defaults.ApplicationLabel: constants.GravityServiceName,
				},
				Annotations: map[string]string{
					AWSIdleTimeoutKey: AWSLoadBalancerIdleTimeout,
					AWSInternalKey:    AWSLoadBalancerInternal,
				},
				Spec: ControllerServiceSpec{
					Type: LoadBalancer,
					Ports: []Port{
						{
							Name:     constants.GravityServicePortName,
							Port:     defaults.GravityServicePort,
							NodePort: defaults.GravitySiteNodePort,
						},
					},
				},
			},
			comment: "initialize service using default controller service config",
		},
		{
			existingServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-existing": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					AWSIdleTimeoutKey: "1",
					AWSInternalKey:    "0.0.0.0/0",
				},
				Spec: ControllerServiceSpec{
					Type: LoadBalancer,
					Ports: []Port{
						{
							Name:     "port-existing",
							Port:     3001,
							NodePort: 32001,
						},
					},
				},
			},
			expectedServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-existing": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					AWSIdleTimeoutKey: "1",
					AWSInternalKey:    "0.0.0.0/0",
				},
				Spec: ControllerServiceSpec{
					Type: LoadBalancer,
					Ports: []Port{
						{
							Name:     "port-existing",
							Port:     3001,
							NodePort: 32001,
						},
					},
				},
			},
			comment: "existing controller service config is not over written by default",
		},
		{
			updatedServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-updated": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					"cloud.google.com/load-balancer-type": "Internal",
				},
				Spec: ControllerServiceSpec{
					Type: NodePort,
					Ports: []Port{
						{
							Name:     "port-updated",
							Port:     3001,
							NodePort: 32001,
						},
					},
				},
			},
			expectedServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-updated": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					"cloud.google.com/load-balancer-type": "Internal",
				},
				Spec: ControllerServiceSpec{
					Type: NodePort,
					Ports: []Port{
						{
							Name:     "port-updated",
							Port:     3001,
							NodePort: 32001,
						},
					},
				},
			},
			comment: "initialize service using updated controller service config",
		},
		{
			existingServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-existing": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					AWSIdleTimeoutKey: AWSLoadBalancerIdleTimeout,
					AWSInternalKey:    AWSLoadBalancerInternal,
				},
				Spec: ControllerServiceSpec{
					Type: LoadBalancer,
					Ports: []Port{
						{
							Name:     "port-existing",
							Port:     defaults.GravityServicePort,
							NodePort: defaults.GravitySiteNodePort,
						},
					},
				},
			},
			updatedServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-updated": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					"cloud.google.com/load-balancer-type": "Internal",
				},
				Spec: ControllerServiceSpec{
					Type: NodePort,
					Ports: []Port{
						{
							Name:     "port-updated",
							Port:     3001,
							NodePort: 32001,
						},
					},
				},
			},
			expectedServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-updated": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					"cloud.google.com/load-balancer-type": "Internal",
				},
				Spec: ControllerServiceSpec{
					Type: NodePort,
					Ports: []Port{
						{
							Name:     "port-updated",
							Port:     3001,
							NodePort: 32001,
						},
					},
				},
			},
			comment: "update existing service",
		},
		{
			existingServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-existing": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					AWSIdleTimeoutKey: AWSLoadBalancerIdleTimeout,
					AWSInternalKey:    AWSLoadBalancerInternal,
				},
				Spec: ControllerServiceSpec{
					Type: LoadBalancer,
					Ports: []Port{
						{
							Name:     "port-existing",
							Port:     3001,
							NodePort: 32001,
						},
					},
				},
			},
			updatedServiceConfig: &GravityControllerService{
				Annotations: map[string]string{
					"cloud.google.com/load-balancer-type": "Internal",
				},
				Spec: ControllerServiceSpec{
					Type: NodePort,
				},
			},
			expectedServiceConfig: &GravityControllerService{
				Labels: map[string]string{
					"app-existing": constants.GravityServiceName,
				},
				Annotations: map[string]string{
					"cloud.google.com/load-balancer-type": "Internal",
				},
				Spec: ControllerServiceSpec{
					Type: NodePort,
					Ports: []Port{
						{
							Name:     "port-existing",
							Port:     3001,
							NodePort: 32001,
						},
					},
				},
			},
			comment: "maintain existing spec if undefined in updated service config",
		},
	}
	for _, tc := range testCases {
		comment := Commentf(tc.comment)

		// clusterControl provides control over updated service config.
		clusterControl := r.NewClusterConfigControl(tc.updatedServiceConfig)

		// serviceControl provides control over existing service.
		serviceControl := r.NewServiceControl(tc.existingServiceConfig)

		c.Assert(Reconcile(clusterControl, serviceControl), IsNil, comment)

		serviceConfig, err := serviceControl.Get()
		c.Assert(err, IsNil, comment)
		c.Assert(serviceConfig, compare.DeepEquals, tc.expectedServiceConfig, comment)
	}
}

// mockClusterConfigControl provides mock implementation of ClusterConfigControl.
type mockClusterConfigControl struct {
	resource Resource
}

// NewClusterConfigControl returns a new cluster config control for the provided
// config.
func (r *KubernetesSuite) NewClusterConfigControl(config *GravityControllerService) *mockClusterConfigControl {
	return &mockClusterConfigControl{
		resource: Resource{
			Spec: Spec{
				ComponentConfigs: ComponentConfigs{
					GravityControllerService: config,
				},
			},
		},
	}
}

// Get returns the cluster's ClusterConfiguration resource.
func (r *mockClusterConfigControl) Get() (*Resource, error) {
	return &r.resource, nil
}

// Update updates the cluster's ClusterConfiguration resource.
func (r *mockClusterConfigControl) Update(_ *Resource) error {
	return trace.NotImplemented("not implemented for mockClusterConfigControl")
}

// mockServiceControl provides mock implementation of ServiceControl.
type mockServiceControl struct {
	svc *v1.Service
}

// NewServiceControl returns a new service control constructed from the provided
// config.
func (r *KubernetesSuite) NewServiceControl(config *GravityControllerService) *mockServiceControl {
	return &mockServiceControl{
		svc: newService(config),
	}
}

// Get returns the controller service configuration.
func (r *mockServiceControl) Get() (*GravityControllerService, error) {
	return toServiceConfig(r.svc), nil
}

// Update updates the controller service.
func (r *mockServiceControl) Update(config *GravityControllerService) error {
	if r.svc == nil {
		r.svc = newService(config)
		return nil
	}

	if !shouldUpdate(toServiceConfig(r.svc), config) {
		return nil
	}

	updateService(r.svc, config)
	return nil
}
