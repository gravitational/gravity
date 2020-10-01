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

package validate

import (
	"net"

	"github.com/gravitational/trace"
)

// KubernetesSubnetsFromStrings makes sure that the provided CIDR ranges are valid and can be used as
// pod/service Kubernetes subnets
func KubernetesSubnetsFromStrings(podCIDR, serviceCIDR string) error {
	var podNet, serviceNet *net.IPNet
	var err error

	// make sure the pod subnet is valid
	if podCIDR != "" {
		_, podNet, err = net.ParseCIDR(podCIDR)
		if err != nil {
			return trace.BadParameter(
				"invalid pod subnet: %q", podCIDR)
		}
	}

	// make sure the service subnet is valid
	if serviceCIDR != "" {
		_, serviceNet, err = net.ParseCIDR(serviceCIDR)
		if err != nil {
			return trace.BadParameter(
				"invalid service subnet: %q", serviceCIDR)
		}
	}

	// make sure the subnets do not overlap
	return KubernetesSubnets(podNet, serviceNet)
}

// KubernetesSubnets makes sure that the provided CIDR ranges can be used as
// pod/service Kubernetes subnets
func KubernetesSubnets(podNet, serviceNet *net.IPNet) (err error) {
	if podNet != nil {
		// make sure the pod subnet is valid
		// the pod network should be /22 minimum so k8s can allocate /24 to each node (minimum 3 nodes)
		ones, _ := podNet.Mask.Size()
		if ones > 22 {
			return trace.BadParameter(
				"pod subnet should be a minimum of /22: %q", podNet.String())
		}
	}
	if podNet != nil && serviceNet != nil {
		// make sure the subnets do not overlap
		if podNet.Contains(serviceNet.IP) || serviceNet.Contains(podNet.IP) {
			return trace.BadParameter(
				"pod subnet %q and service subnet %q should not overlap",
				podNet.String(), serviceNet.String())
		}
	}
	return nil
}

// NormalizeSubnet returns the text representation of the given subnet
// as the IP masked with the network mask
func NormalizeSubnet(subnet string) (string, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return ipNet.String(), nil
}
