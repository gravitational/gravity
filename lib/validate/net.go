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
	"strconv"

	"github.com/gravitational/trace"
)

// NetworkOverlap verifies that ipAddr is not in the range of the subnetCIDR
func NetworkOverlap(ipAddr, subnetCIDR, errMsg string) error {
	_, subNet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return trace.Wrap(err)
	}

	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return trace.BadParameter("invalid IP address (%v)", ipAddr)
	}

	if subNet.Contains(ip) {
		return trace.BadParameter(errMsg)
	}

	return nil
}

// KubernetesSubnetsFromStrings makes sure that the provided CIDR ranges are valid and can be used as
// pod/service Kubernetes subnets
func KubernetesSubnetsFromStrings(podCIDR, serviceCIDR, podSubnetSize string) error {
	var podNet, serviceNet *net.IPNet
	var subnetSize int
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

	// make sure podSubnetSize is valid
	if podSubnetSize != "" {
		subnetSize, err = strconv.Atoi(podSubnetSize)
		if err != nil || subnetSize < 1 || subnetSize > 32 {
			return trace.BadParameter("invalid pod subnet size: %q", podSubnetSize)
		}

		// The minimum subnet size accepted by flannel is /28:
		// https://github.com/gravitational/flannel/blob/master/subnet/config.go#L70-L74
		if subnetSize > 28 {
			return trace.BadParameter("pod subnet is too small. Minimum useful network prefix is /28: %q", podSubnetSize)
		}
	}

	// make sure the subnets do not overlap
	return KubernetesSubnets(podNet, serviceNet, subnetSize)
}

// KubernetesSubnets makes sure that the provided CIDR ranges can be used as
// pod/service Kubernetes subnets
func KubernetesSubnets(podNet, serviceNet *net.IPNet, podSubnetSize int) (err error) {
	if podNet == nil {
		return nil
	}

	// make sure the pod subnet is valid
	// the pod network should be /22 minimum so k8s can allocate /24 to each node (minimum 3 nodes)
	ones, _ := podNet.Mask.Size()
	if ones > 22 {
		return trace.BadParameter(
			"pod subnet should be a minimum of /16: %q", podNet.String())
	}

	if serviceNet != nil {
		// make sure the subnets do not overlap
		if podNet.Contains(serviceNet.IP) || serviceNet.Contains(podNet.IP) {
			return trace.BadParameter(
				"pod subnet %q and service subnet %q should not overlap",
				podNet.String(), serviceNet.String())
		}
	}

	if podSubnetSize != 0 {
		// make sure the subnet size is smaller than the pod network CIDR range
		if podSubnetSize < ones {
			return trace.BadParameter("pod subnet size (%d) cannot be larger than the network CIDR range (%q)",
				podSubnetSize, podNet.String())
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
