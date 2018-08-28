// Copyright 2016 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rigging

// types so we can load k8s JSON

type Job struct {
	Status JobStatus
}

type JobStatus struct {
	Succeeded int
	Active    int
}

type ReplicationController struct {
	Status ReplicationControllerStatus
}

type ReplicationControllerStatus struct {
	Replicas             int
	FullyLabeledReplicas int
	ObservedGeneration   int
}

type PodCondition struct {
	Type   string
	Status string
}

type PodStatus struct {
	Phase      string
	Conditions []PodCondition
}

type Pod struct {
	Status PodStatus
}

type PodList struct {
	Items []Pod
}

type Node struct {
	Metadata Metadata
}

type NodeList struct {
	Items []Node
}

type Metadata struct {
	Name   string
	Labels map[string]string
}
