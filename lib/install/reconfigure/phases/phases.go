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

package phases

const (
	// LocalPackagesPhase removes old local packages.
	LocalPackagesPhase = "/local-packages"
	// ClusterPackagesPhase removes old cluster packages.
	ClusterPackagesPhase = "/cluster-packages"
	// DirectoriesPhase cleans up some directories.
	DirectoriesPhase = "/directories"
	// EtcdPhase updates etcd member's peer advertise URL.
	EtcdPhase = "/etcd"
	// StatePhase updates the cluster state.
	StatePhase = "/state"
	// NetworkPhase removes old network interfaces.
	NetworkPhase = "/interfaces"
	// TokensPhase removes old service account tokens.
	TokensPhase = "/tokens"
	// NodePhase removes old Kubernetes node object.
	NodePhase = "/node"
	// PodsPhase removes old Kubernetes pods.
	PodsPhase = "/pods"
	// GravityPhase waits for gravity-site API to become available.
	GravityPhase = "/gravity"
	// RestartPhase encapsulates Teleport/Planet restart subphases.
	RestartPhase = "/restart"
	// TeleportPhase restarts Teleport unit.
	TeleportPhase = "/teleport"
	// PlanetPhase restart Planet unit.
	PlanetPhase = "/planet"
)
