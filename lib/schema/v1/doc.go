// https://blog.openshift.com/kubernetes-deep-dive-code-generation-customresources/
// +k8s:deepcopy-gen=package,register
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Package v1 is the v1 version of the API.
// +groupName=gravitational.io
package v1
