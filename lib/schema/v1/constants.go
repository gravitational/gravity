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

package v1

const (
	// KindApplication is the legacy kind for a user application
	KindApplication = "Application"

	// KindSystemApplication is the legacy kind for a system application
	KindSystemApplication = "SystemApplication"

	// KindRuntime is the legacy kind for a runtime application
	KindRuntime = "Runtime"

	// ServiceRoleMaster names a label that defines a master node role
	ServiceRoleMaster ServiceRole = "master"

	// ServiceRoleNode names a label that defines a regular node role
	ServiceRoleNode ServiceRole = "node"

	// Version specifies the package version
	Version = "v1"

	// RuntimePackageName names the runtime package with extended AWS configuration
	RuntimePackageName = "k8s-aws"

	// RuntimeOnPremPackageName names the runtime package for onprem configuration
	RuntimeOnPremPackageName = "k8s-onprem"
)

// ServiceRole defines the type for the node service role
type ServiceRole string
