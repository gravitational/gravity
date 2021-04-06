/*
Copyright 2019 Gravitational, Inc.

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

package checks

import (
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// Requirements defines a set of requirements for a node profile.
type Requirements struct {
	// CPU describes CPU requirements.
	CPU *schema.CPU
	// RAM describes RAM requirements.
	RAM *schema.RAM
	// OS describes OS requirements
	OS []schema.OS
	// Network describes network requirements.
	Network Network
	// Volumes describes volumes requirements.
	Volumes []schema.Volume
	// Docker describes Docker requirements.
	Docker storage.DockerConfig
}

// Network describes network requirements.
type Network struct {
	// MinTransferRate is minimum required transfer rate.
	MinTransferRate utils.TransferRate
	// Ports specifies requirements for ports to be available on server.
	Ports Ports
}

// Ports describes port requirements for a specific profile.
type Ports struct {
	// TCP lists a range of TCP ports.
	TCP []int
	// UDP lists a range of UDP ports.
	UDP []int
}

// RequirementsFromManifest returns check requirements for each node profile
// in the provided manifest.
func RequirementsFromManifest(manifest schema.Manifest) (map[string]Requirements, error) {
	result := make(map[string]Requirements)
	for i, profile := range manifest.NodeProfiles {
		tcp, udp, err := profile.Ports()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req := Requirements{
			CPU:     &manifest.NodeProfiles[i].Requirements.CPU,
			RAM:     &manifest.NodeProfiles[i].Requirements.RAM,
			OS:      profile.Requirements.OS,
			Volumes: profile.Requirements.Volumes,
			Network: Network{
				MinTransferRate: profile.Requirements.Network.MinTransferRate,
				Ports:           Ports{TCP: tcp, UDP: udp},
			},
		}
		result[profile.Name] = req
	}
	return result, nil
}

// RequirementsFromManifests generates check requirements as a difference
// between two manifests - old and new.
func RequirementsFromManifests(old, new schema.Manifest, profiles map[string]string, docker storage.DockerConfig) (map[string]Requirements, error) {
	result := make(map[string]Requirements)
	for _, profileName := range profiles {
		oldProfile, err := old.NodeProfiles.ByName(profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		newProfile, err := new.NodeProfiles.ByName(profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Compute port requirements for this profile
		tcp, udp, err := schema.DiffPorts(old, new, profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req := Requirements{
			OS:      newProfile.Requirements.OS,
			Volumes: schema.DiffVolumes(oldProfile.Requirements.Volumes, newProfile.Requirements.Volumes),
			Network: Network{
				Ports: Ports{TCP: tcp, UDP: udp},
			},
			Docker: docker,
		}
		result[profileName] = req
	}
	return result, nil
}
