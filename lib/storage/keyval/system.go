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

package keyval

import (
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// GetDNSConfig returns current cluster local DNS configuration
func (b *backend) GetDNSConfig() (*storage.DNSConfig, error) {
	var config storage.DNSConfig
	err := b.getVal(b.key(systemP, dnsP), &config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &config, nil
}

// SetDNSConfig sets current cluster local DNS configuration
func (b *backend) SetDNSConfig(config storage.DNSConfig) error {
	err := b.upsertVal(b.key(systemP, dnsP), &config, forever)
	return trace.Wrap(err)
}

// GetSELinux returns whether SELinux support is on
func (b *backend) GetSELinux() (enabled bool, err error) {
	if err := b.getVal(b.key(systemP, seLinuxP), &enabled); err != nil {
		return false, trace.Wrap(err)
	}
	return enabled, nil
}

// SetSELinux sets SELinux support
func (b *backend) SetSELinux(enabled bool) error {
	return b.upsertVal(b.key(systemP, seLinuxP), &enabled, forever)
}
