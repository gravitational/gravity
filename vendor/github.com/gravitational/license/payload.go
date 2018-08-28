/*
Copyright 2017 Gravitational, Inc.

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

package license

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gravitational/license/constants"

	sigar "github.com/cloudfoundry/gosigar"
	"github.com/gravitational/trace"
)

// Payload is custom information that gets encoded into licenses.
//
// For some customers JSON-formatted payload serves as a license itself.
type Payload struct {
	// ClusterID is vendor-specific cluster ID
	ClusterID string `json:"cluster_id,omitempty"`
	// Expiration is expiration time for the license
	Expiration time.Time `json:"expiration,omitempty"`
	// MaxNodes is maximum number of nodes the license allows
	MaxNodes int `json:"max_nodes,omitempty"`
	// MaxCores is maximum number of CPUs per node the license allows
	MaxCores int `json:"max_cores,omitempty"`
	// Company is the company name the license is generated for
	Company string `json:"company,omitempty"`
	// Person is the name of the person the license is generated for
	Person string `json:"person,omitempty"`
	// Email is the email of the person the license is generated for
	Email string `json:"email,omitempty"`
	// Metadata is an arbitrary customer metadata
	Metadata string `json:"metadata,omitempty"`
	// ProductName is the name of the product the license is for
	ProductName string `json:"product_name,omitempty"`
	// ProductVersion is the product version
	ProductVersion string `json:"product_version,omitempty"`
	// EncryptionKey is the passphrase for decoding encrypted packages
	EncryptionKey []byte `json:"encryption_key,omitempty"`
	// Signature is vendor-specific signature
	Signature string `json:"signature,omitempty"`
	// Shutdown indicates whether the app should be stopped when the license expires
	Shutdown bool `json:"shutdown,omitempty"`
}

// UnmarshalJSON is a custom JSON unmarshaler for payload.
func (p *Payload) UnmarshalJSON(data []byte) error {
	// first try to unmarshal as a "normal" payload object which is
	// how we store our own license details
	var unmarshaled payload
	err := json.Unmarshal(data, &unmarshaled)
	if err == nil {
		*p = Payload(unmarshaled)
		return nil
	}
	// if that failed, use custom unmarshaler with the format defined
	// below that some of vendors use
	var aux unmarshalFormat
	err = json.Unmarshal(data, &aux)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Expiration, err = time.Parse(constants.LicenseTimeFormat, aux.Expiration)
	if err != nil {
		return trace.Wrap(err)
	}
	p.MaxNodes = 0
	if aux.MaxNodes != "" {
		p.MaxNodes, err = strconv.Atoi(aux.MaxNodes)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	p.MaxCores = 0
	if aux.MaxCores != "" {
		p.MaxCores, err = strconv.Atoi(aux.MaxCores)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	p.ClusterID = aux.ClusterID
	p.Company = aux.Company
	p.Person = aux.Person
	p.Email = aux.Email
	p.Signature = aux.Signature
	p.Shutdown = aux.Shutdown
	return nil
}

// payload is used to avoid recursion when unmarshaling
type payload Payload

// unmarshalFormat specifies how to unmarshal license payload from a JSON string.
//
// It's needed because payload serves as a license itself for some customers and they
// have strict rules of what the string version of license should look like.
type unmarshalFormat struct {
	ClusterID  string `json:"cluster_id,omitempty"`
	Expiration string `json:"expiration,omitempty"`
	MaxNodes   string `json:"maxnodes,omitempty"`
	MaxCores   string `json:"maxcores,omitempty"`
	Company    string `json:"company,omitempty"`
	Person     string `json:"person,omitempty"`
	Email      string `json:"email,omitempty"`
	Signature  string `json:"signature,omitempty"`
	Shutdown   bool   `json:"shutdown,omitempty"`
}

// parsePayload attemps to parse the provided license string as a license payload.
func parsePayload(license string) (License, error) {
	var p Payload
	err := json.Unmarshal([]byte(license), &p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

// Verify verifies that the expiration time is not in the past.
func (p Payload) Verify(_ []byte) error {
	if p.Expiration.Before(time.Now().UTC()) {
		return trace.BadParameter("the license has expired")
	}
	return nil
}

// GetPayload returns payload.
func (p Payload) GetPayload() Payload {
	return p
}

// CheckCount checks if the license supports the provided number of servers.
func (p Payload) CheckCount(count int) error {
	if p.MaxNodes != 0 && count > p.MaxNodes {
		return trace.BadParameter(
			"the license allows maximum of %v nodes, requested: %v", p.MaxNodes, count)
	}
	return nil
}

// CheckCPU checks if the license supports the provided number of CPUs.
func (p Payload) CheckCPU(cpu sigar.CpuList) error {
	count := len(cpu.List)
	if p.MaxCores != 0 && count > p.MaxCores {
		return trace.BadParameter(
			"the license allows maximum of %v CPUs, requested: %v", p.MaxCores, count)
	}
	return nil
}

// CheckInstanceTypes checks if the license supports all of the provided AWS instance types.
func (p Payload) CheckInstanceTypes(instanceTypes []string) error {
	supported := make(map[string]struct{})
	for _, t := range p.FilterInstanceTypes(instanceTypes) {
		supported[t] = struct{}{}
	}
	for _, t := range instanceTypes {
		if _, ok := supported[t]; !ok {
			return trace.BadParameter(
				"the license does not support instance type: %v", t)
		}
	}
	return nil
}

// FilterInstanceTypes retuns a subset of the provided AWS instance types supported by the license.
func (p Payload) FilterInstanceTypes(instanceTypes []string) []string {
	if p.MaxCores == 0 {
		return instanceTypes
	}
	supported := []string{}
	for _, t := range instanceTypes {
		for name, cores := range constants.EC2InstanceTypes {
			if name == t && cores <= p.MaxCores {
				supported = append(supported, t)
			}
		}
	}
	return supported
}
