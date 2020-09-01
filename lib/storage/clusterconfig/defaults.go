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
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AWSIdleTimeoutKey defines the aws load balancer idle timeout property name
	AWSIdleTimeoutKey = "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout"

	// AWSInternalKey defines the aws load balancer internal property name
	AWSInternalKey = "service.beta.kubernetes.op/aws-load-balancer-internal"

	// AWSLoadBalancerIdleTimeoutSeconds defines the default aws load balancer idle timeout in seconds
	AWSLoadBalancerIdleTimeoutSeconds = "3600"

	// AWSLoadBalancerInternal defines the default aws load balancer internal
	AWSLoadBalancerInternal = "0.0.0.0/0"
)

// ClusterConfigMap returns the default cluster ConfigMap.
func ClusterConfigMap() *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindConfigMap,
			APIVersion: metav1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        constants.ClusterConfigurationMap,
			Namespace:   defaults.KubeSystemNamespace,
			Annotations: map[string]string{},
		},
		Data: map[string]string{},
	}
}

// ControllerService returns the default controller service.
func ControllerService() *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindService,
			APIVersion: constants.ServiceAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GravityServiceName,
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				defaults.ApplicationLabel: constants.GravityServiceName,
			},
			Annotations: map[string]string{
				AWSIdleTimeoutKey: AWSLoadBalancerIdleTimeoutSeconds,
				AWSInternalKey:    AWSLoadBalancerInternal,
			},
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceType(LoadBalancer),
			Ports: []v1.ServicePort{
				{
					Name:     constants.GravityServicePortName,
					Port:     defaults.GravityServicePort,
					NodePort: defaults.GravitySiteNodePort,
				},
			},
			Selector: map[string]string{
				defaults.ApplicationLabel: constants.GravityServiceName,
			},
		},
	}
}
