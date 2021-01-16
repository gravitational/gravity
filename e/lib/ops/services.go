// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ops

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServicesFromEndpoints returns Kubernetes specs for user and cluster traffic
// services based on the provided cluster endpoints
func ServicesFromEndpoints(endpoints storage.Endpoints) (publicService *v1.Service, agentsService *v1.Service, err error) {
	publicAddr, err := utils.NewAddress(endpoints.GetPublicAddr())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	agentsAddr, err := utils.NewAddress(endpoints.GetAgentsAddr())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return getPublicService(*publicAddr, *agentsAddr),
		getAgentsService(*publicAddr, *agentsAddr), nil
}

// getPublicService returns a spec of Kubernetes service for user traffic
// based on the provided endpoints
func getPublicService(publicAddr, agentsAddr utils.Address) *v1.Service {
	service := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindService,
			APIVersion: constants.ServiceAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GravityPublicService,
			Namespace: defaults.KubeSystemNamespace,
			Labels: map[string]string{
				defaults.ApplicationLabel: defaults.GravityOpsCenterLabel,
			},
			Annotations: map[string]string{
				constants.AWSLBIdleTimeoutAnnotation: defaults.LBIdleTimeout,
				constants.ExternalDNSHostnameAnnotation: constants.ExternalDNS(
					publicAddr.Addr),
			},
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeLoadBalancer,
			Selector: defaults.GravitySiteSelector,
		},
	}
	// if traffic is not separated, the service exposes a single port for
	// gravity traffic as well as teleport ports
	if publicAddr.Equal(agentsAddr) {
		service.Spec.Ports = append(service.Spec.Ports,
			// all gravity traffic is served on this port
			v1.ServicePort{
				Name: "public",
				Port: publicAddr.Port,
				// there will be 1 listener that serves everything on
				// the default 3009 port
				TargetPort: intstr.FromInt(defaults.GravityListenPort),
			},
			// teleport reverse tunnel service is listening on this port
			v1.ServicePort{
				Name: "sshtunnel",
				Port: teledefaults.SSHProxyTunnelListenPort,
			},
			// teleport proxy service is listening on this port
			v1.ServicePort{
				Name: "sshproxy",
				Port: teledefaults.SSHProxyListenPort,
			},
			// teleport kube proxy service is listening on this port
			v1.ServicePort{
				Name: "kubeproxy",
				Port: teledefaults.KubeProxyListenPort,
			})
		return service
	}
	// if traffic is separated by port but hostname is the same, this service
	// will expose two ports - one for user traffic and another for cluster,
	// as well as teleport ports
	if publicAddr.EqualAddr(agentsAddr) {
		service.Spec.Ports = append(service.Spec.Ports,
			// gravity user traffic is served on this port
			v1.ServicePort{
				Name: "public",
				Port: publicAddr.Port,
				// there will be a separate listener that will serve public
				// traffic on port 3007
				TargetPort: intstr.FromInt(defaults.GravityPublicListenPort),
			},
			// gravity cluster traffic is service on this port
			v1.ServicePort{
				Name: "agents",
				Port: agentsAddr.Port,
				// all internal API will be served on the default 3009 port
				TargetPort: intstr.FromInt(defaults.GravityListenPort),
			},
			// teleport reverse tunnel service is listening on this port
			v1.ServicePort{
				Name: "sshtunnel",
				Port: teledefaults.SSHProxyTunnelListenPort,
			},
			// teleport proxy service is listening on this port
			v1.ServicePort{
				Name: "sshproxy",
				Port: teledefaults.SSHProxyListenPort,
			},
			// teleport kube proxy service is listening on this port
			v1.ServicePort{
				Name: "kubeproxy",
				Port: teledefaults.KubeProxyListenPort,
			})
		return service
	}
	// otherwise, this service will expose 1 port for public traffic and
	// there will be a separate service for agents traffic
	service.Spec.Ports = append(service.Spec.Ports,
		// gravity user traffic is served on this port
		v1.ServicePort{
			Name: "public",
			Port: publicAddr.Port,
			// there will be a separate listener that will serve public
			// traffic on port 3007
			TargetPort: intstr.FromInt(defaults.GravityPublicListenPort),
		},
		// teleport proxy service is listening on this port
		v1.ServicePort{
			Name: "sshproxy",
			Port: teledefaults.SSHProxyListenPort,
		},
		// teleport kube proxy service is listening on this port
		v1.ServicePort{
			Name: "kubeproxy",
			Port: teledefaults.KubeProxyListenPort,
		})
	return service
}

// getAgentsService returns a spec of Kubernetes service for cluster
// traffic based on the provided endpoints
func getAgentsService(publicAddr, agentsAddr utils.Address) *v1.Service {
	service := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindService,
			APIVersion: constants.ServiceAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GravityAgentsService,
			Namespace: defaults.KubeSystemNamespace,
			Labels: map[string]string{
				defaults.ApplicationLabel: defaults.GravityOpsCenterLabel,
			},
			Annotations: map[string]string{
				constants.AWSLBIdleTimeoutAnnotation: defaults.LBIdleTimeout,
			},
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeLoadBalancer,
			Selector: defaults.GravitySiteSelector,
		},
	}
	// if there's no traffic separation or it's separated only by port (but
	// hostname is the same), there will be only "gravity-public" service, so
	// return a service without ports here to signal to the caller that this
	// service should be deleted if it's present
	if publicAddr.EqualAddr(agentsAddr) {
		return service
	}
	// otherwise this service should forward traffic to the listener that
	// serves cluster traffic
	service.Spec.Ports = append(service.Spec.Ports,
		// gravity cluster traffic is served on this port
		v1.ServicePort{
			Name: "agents",
			Port: agentsAddr.Port,
			// all internal API will be served on the default 3009 port
			TargetPort: intstr.FromInt(defaults.GravityListenPort),
		},
		// teleport reverse tunnel service is listening on this port
		v1.ServicePort{
			Name: "sshtunnel",
			Port: teledefaults.SSHProxyTunnelListenPort,
		})
	return service
}
