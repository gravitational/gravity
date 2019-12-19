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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageSet contains references of images managed by the lens controller.
type ImageSet struct {
	// TypeMeta is the resource type metadata.
	metav1.TypeMeta `json:",inline"`
	// ObjectMeta is the resource metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the image set specification.
	Spec ImageSetSpec `json:"spec"`
	// Status contains image set runtime information.
	Status ImageSetStatus `json:"status"`
}

//ImageSetSpec contains a list of Docker image references.
type ImageSetSpec struct {
	// Images is a list of image references.
	Images []ImageSetImage `json:"images"`
}

// ImageSetImage represents a single image reference.
type ImageSetImage struct {
	// Image is the image reference.
	Image string `json:"image"`
	// Registry is an optional registry address.
	Registry string `json:"registry,omitempty"`
}

// ImageSetStatus contains image set runtime information.
type ImageSetStatus struct{}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageSetList is a list of image sets.
type ImageSetList struct {
	// TypeMeta is the resource type metadata.
	metav1.TypeMeta `json:",inline"`
	// ListMeta is the resource metadata.
	metav1.ListMeta `json:"metadata"`
	// Items is a list of image sets.
	Items []ImageSet `json:"items"`
}
