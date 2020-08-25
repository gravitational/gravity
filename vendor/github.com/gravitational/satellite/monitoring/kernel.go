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

package monitoring

import "fmt"

// KernelVersion describes an abbreviated version of a Linux kernel.
// It contains the kernel version (including major/minor components) and
// patch number
//
// Example:
//  $ uname -r
//  $ 4.4.9-112-generic
//
// The result will be:
//  KernelVersion{Release: 4, Major: 4, Minor: 9, Patch: 112}
type KernelVersion struct {
	// Release specifies the release of the kernel
	Release int
	// Major specifies the major version component
	Major int
	// Minor specifies the minor version component
	Minor int
	// Patch specifies the patch or build number
	Patch int
}

// String returns the kernel version formatted as Release.Major.Minor-Patch.
func (r *KernelVersion) String() string {
	return fmt.Sprintf("%d.%d.%d-%d", r.Release, r.Major, r.Minor, r.Patch)
}
