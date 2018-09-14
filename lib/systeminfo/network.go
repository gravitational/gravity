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

package systeminfo

import (
	"net"

	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// HasInterface returns nil if the machine the method executes on has the specified network interface
func HasInterface(addr string) error {
	netIfaces, err := net.Interfaces()
	if err != nil {
		return trace.Wrap(err)
	}

	ifaces, err := networkInterfaces(netIfaces)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, iface := range ifaces {
		if iface.IPv4 == addr {
			return nil
		}
	}
	return trace.NotFound("interface %q not found on this machine", addr)
}

// NetworkInterfaces returns the list of all network interfaces with IPv4 addresses on the host
func NetworkInterfaces() (result []storage.NetworkInterface, err error) {
	netIfaces, err := net.Interfaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ifaces, err := networkInterfaces(netIfaces)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, iface := range ifaces {
		result = append(result, iface)
	}
	return result, nil
}

// networkInterfaces returns the list of all network interfaces with IPv4 addresses on the host
func networkInterfaces(ifaces []net.Interface) (result map[string]storage.NetworkInterface, err error) {
	getIPv4 := func(addrs []net.Addr) net.IP {
		for _, ifaddr := range addrs {
			switch ipnet := ifaddr.(type) {
			case *net.IPNet:
				v4 := ipnet.IP.To4()
				if len(v4) != 0 {
					return v4
				}
			}
		}
		return nil
	}

	result = make(map[string]storage.NetworkInterface)
	for _, iface := range ifaces {
		if iface.Name[:2] == "lo" {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ip := getIPv4(addrs)
		// only record interfaces that have IPv4 addresses present
		if len(ip) != 0 {
			result[iface.Name] = storage.NetworkInterface{
				Name: iface.Name,
				IPv4: ip.String(),
			}
		}
	}
	return result, nil
}
