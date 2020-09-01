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
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// toPorts converts the list of kubernetes servicePorts to a list of Ports.
func toPorts(servicePorts []v1.ServicePort) []Port {
	ports := make([]Port, len(servicePorts))
	for i, sPort := range servicePorts {
		ports[i] = Port{
			Name:       sPort.Name,
			Protocol:   string(sPort.Protocol),
			Port:       sPort.Port,
			TargetPort: sPort.TargetPort.StrVal,
			NodePort:   sPort.NodePort,
		}
	}
	return ports
}

// toServicePorts converts the list of Ports to a list of kubernetes servicePorts.
func toServicePorts(ports []Port) []v1.ServicePort {
	servicePorts := make([]v1.ServicePort, len(ports))
	for i, port := range ports {
		servicePorts[i] = v1.ServicePort{
			Name:       port.Name,
			Protocol:   v1.Protocol(port.Protocol),
			Port:       port.Port,
			TargetPort: intstr.Parse(port.TargetPort),
			NodePort:   port.NodePort,
		}
	}
	return servicePorts
}

// hasDiff returns true if the two provided controller service configs have
// diverged.
func hasDiff(existing, incoming *GravityControllerService) bool {
	if len(existing.Labels) != len(incoming.Labels) {
		return true
	}
	for key, incomingVal := range incoming.Labels {
		existingVal, exists := existing.Labels[key]
		if !exists || existingVal != incomingVal {
			return true
		}
	}

	if len(existing.Annotations) != len(incoming.Annotations) {
		return true
	}
	for key, incomingVal := range incoming.Annotations {
		existingVal, exists := existing.Annotations[key]
		if !exists || existingVal != incomingVal {
			return true
		}
	}

	if existing.Spec.Type != incoming.Spec.Type {
		return true
	}

	if len(existing.Spec.Ports) != len(incoming.Spec.Ports) {
		return true
	}

	// Sort ports before comparing.
	sort.Sort(ByName(existing.Spec.Ports))
	sort.Sort(ByName(incoming.Spec.Ports))

	for i, incomingPort := range incoming.Spec.Ports {
		existingPort := existing.Spec.Ports[i]
		if existingPort != incomingPort {
			return true
		}
	}

	return false
}

// ByName implements sort.Interface based on the Name field.
type ByName []Port

func (r ByName) Len() int           { return len(r) }
func (r ByName) Less(i, j int) bool { return r[i].Name < r[j].Name }
func (r ByName) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
