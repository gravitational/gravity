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

package constants

const (
	// ComponentOpsCenter represents the name of the mode gravity process
	// is running in in an Ops Center cluster
	ComponentOpsCenter = "opscenter"
	// OpsConfigMapName is a name of the opscenter configmap
	OpsConfigMapName = "gravity-opscenter"
	// OpsConfigMapTeleport is a K8s Config map teleport.yaml file property
	OpsConfigMapTeleport = "teleport.yaml"
	// OpsConfigMapGravity is a K8s Config map gravity.yaml file property
	OpsConfigMapGravity = "gravity.yaml"
)
