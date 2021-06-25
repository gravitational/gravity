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

package schema

import (
	"sort"

	"github.com/gravitational/trace"
	"github.com/xtgo/set"
)

// DiffPorts returns a difference of port requirements between old and new
// for the specified profile.
func DiffPorts(old, new Manifest, profileName string) (tcp, udp []int, err error) {
	profile, err := old.NodeProfiles.ByName(profileName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	oldTCP, oldUDP, err := profile.Ports()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sort.Ints(oldTCP)
	sort.Ints(oldUDP)

	newProfile, err := new.NodeProfiles.ByName(profileName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tcp, udp, err = newProfile.Ports()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sort.Ints(tcp)
	sort.Ints(udp)

	commonTCP := append(oldTCP, tcp...)
	commonUDP := append(oldUDP, udp...)

	// Do not take ports that are only present in the old
	// manifest into account
	sizeTCP := set.Inter(sort.IntSlice(commonTCP), len(oldTCP))
	sizeUDP := set.Inter(sort.IntSlice(commonUDP), len(oldUDP))
	commonTCP = commonTCP[:sizeTCP]
	commonUDP = commonUDP[:sizeUDP]

	// Compute the difference
	tcp = append(commonTCP, tcp...)
	udp = append(commonUDP, udp...)
	sizeTCP = set.SymDiff(sort.IntSlice(tcp), len(commonTCP))
	sizeUDP = set.SymDiff(sort.IntSlice(udp), len(commonUDP))
	return tcp[:sizeTCP], udp[:sizeUDP], nil
}

// DiffVolumes returns a difference between old and new volume requirements.
func DiffVolumes(old, new []Volume) []Volume {
	volumes := append(old, new...)
	size := set.Inter(volumesByPath(volumes), len(old))
	// Compute common volumes
	common := volumes[:size]
	// Compute volumes only present in new
	volumes = append(common, new...)
	size = set.SymDiff(volumesByPath(volumes), len(common))
	return volumes[:size]
}

type volumesByPath []Volume

func (r volumesByPath) Len() int           { return len(r) }
func (r volumesByPath) Less(i, j int) bool { return r[i].Path < r[j].Path }
func (r volumesByPath) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
