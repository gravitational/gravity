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

package schema

import "fmt"

const (
	// LabelRole names a label that defines a node role
	LabelRole = "role"

	// ServiceLabelRole names a label that defines a kubernetes node role
	ServiceLabelRole = "gravitational.io/k8s-role"

	// ProviderAWS defines AWS as infrastructure provider
	ProviderAWS = "aws"
	// ProviderGeneric defines a generic provider
	ProviderGeneric = "generic"
	// ProviderOnPrem defines an On-Premises infrastructure provider
	ProviderOnPrem = "onprem"
	// ProviderGCE defines Google Compute Engine provider
	ProviderGCE = "gce"

	// ProvisionerAWSTerraform defines an operation provisioner based on terraform
	ProvisionerAWSTerraform = "aws_terraform"
	// ProvisionerOnPrem defines a provisioner for an On-Premises operation
	ProvisionerOnPrem = "onprem"

	// NetworkingAWSVPC defines a type of networking for AWS based on AWS-VPC
	NetworkingAWSVPC = "aws-vpc"
	// NetworkingCalico defines a type of networking using Calico
	NetworkingCalico = "calico"
	// NetworkingFlannel defines a type of networking using Flannel VXLAN
	NetworkingFlannel = "vxlan"

	// DisplayRole defines a role used to identify a server instance in the inventory
	// management console
	DisplayRole = "display-role"

	// SystemDevice defines the name of the agent download URI query parameter for system (state) device
	SystemDevice = "system_device"

	// DockerDevice defines the name of the agent download URI query parameter for docker devicemapper device
	DockerDevice = "docker_device"

	// AdvertiseAddr is advertise IP address used in agents
	AdvertiseAddr = "advertise_addr"

	// MountSpec defines the name of the agent download URI query parameter for a mount specification
	MountSpec = "mount"

	// GCENodeTags defines the name of the agent download URI query parameter to override instance node tags
	// on GCE
	GCENodeTags = "gce_node_tags"

	// KindBundle defines a user application type
	KindBundle = "Bundle"
	// KindSystemApplication defines a system application type
	KindSystemApplication = "SystemApplication"
	// KindRuntime defines a runtime application type
	KindRuntime = "Runtime"

	// APIVersionV1 specifies the previous API version
	APIVersionV1 = "v1"
	// APIVersionLegacyV2 specifies legacy v2 version
	APIVersionLegacyV2 = "v2"

	// GroupName specifies the name of the group for application manifest package
	GroupName = "bundle.gravitational.io"
	// Version specifies the current package version
	Version = "v2"

	// ExpandPolicyFixed is a node membership policy that prevents adding
	// more nodes of the same role
	ExpandPolicyFixed = "fixed"
	// ExpandPolicyFixedInstance is a node membership policy that allows adding
	// nodes of the same role with the same instance type only (for cloud
	// providers)
	ExpandPolicyFixedInstance = "fixed-instance"

	// OpsCenterAppName is the name of the Ops Center application
	OpsCenterAppName = "opscenter"
	// OpsCenterNode is the Ops Center app node profile name
	OpsCenterNode = "node"
	// OpsCenterFlavor is the Ops Center app flavor
	OpsCenterFlavor = "single"
)

// ServiceRole defines the type for the node service role
type ServiceRole string

const (
	// ServiceRoleMaster names a label that defines a master node role
	ServiceRoleMaster ServiceRole = "master"

	// ServiceRoleNode names a label that defines a regular node role
	ServiceRoleNode ServiceRole = "node"

	// ApplicationDefaultNamespace defines the default application manifest
	ApplicationDefaultNamespace = "default"
)

// APIVersionV2 specifies the current API version
var APIVersionV2 = fmt.Sprintf("%v/%v", GroupName, Version)

// SupportedProvider is a list of currently supported providers
var SupportedProviders = []string{
	ProviderGeneric,
	ProviderAWS,
	ProviderGCE,
}
