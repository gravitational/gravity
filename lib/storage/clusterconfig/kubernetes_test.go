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
	"context"
	"time"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/trace"

	. "gopkg.in/check.v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
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
					AWSIdleTimeoutKey: AWSLoadBalancerIdleTimeoutSeconds,
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
					AWSIdleTimeoutKey: AWSLoadBalancerIdleTimeoutSeconds,
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
					AWSIdleTimeoutKey: AWSLoadBalancerIdleTimeoutSeconds,
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

		ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
		defer cancel()

		client, err := r.NewClient(ctx, tc.existingServiceConfig, tc.updatedServiceConfig)
		c.Assert(err, IsNil, comment)

		clusterControl := NewClusterConfigControl(client)
		serviceControl := NewServiceControl(client)

		c.Assert(Reconcile(ctx, clusterControl, serviceControl), IsNil, comment)

		serviceConfig, err := serviceControl.Get()
		c.Assert(err, IsNil, comment)
		c.Assert(serviceConfig, compare.DeepEquals, tc.expectedServiceConfig, comment)
	}
}

// NewClient initializes a fake clientset. The provided existing config is used
// to initialize the controller service. The provided incoming config is used to
// initialize the cluster configmap.
func (r *KubernetesSuite) NewClient(ctx context.Context,
	existing *GravityControllerService,
	incoming *GravityControllerService) (kubernetes.Interface, error) {
	client := fake.NewSimpleClientset(
		ClusterConfigMap(),
		ControllerService(),
	)
	config := newEmpty()
	config.Spec.GravityControllerService = incoming
	if err := NewClusterConfigControl(client).Update(ctx, config); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := NewServiceControl(client).Update(ctx, existing); err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}
