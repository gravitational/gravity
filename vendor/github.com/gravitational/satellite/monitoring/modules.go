// +build !linux

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

package monitoring

import "github.com/gravitational/satellite/agent/health"

// NewKernelModuleChecker creates a new kernel module checker
func NewKernelModuleChecker(modules ...ModuleRequest) health.Checker {
	return noopChecker{}
}

// ModuleRequest describes a kernel module
type ModuleRequest struct {
	// Name names the kernel module
	Name string `json:"name"`
	// Names lists alternative names for the module if any.
	// For example, on CentOS 7.2 bridge netfilter module is called "bridge"
	// instead of "br_netfilter".
	Names []string `json:"names,omitempty"`
}

// KernelModuleCheckerID is the ID of the checker of kernel modules
const KernelModuleCheckerID = "kernel-module"

// KernelModuleCheckerData gets attached to the kernel module check probes
type KernelModuleCheckerData struct {
	// Module is the probed kernel module
	Module ModuleRequest `json:"module"`
}
