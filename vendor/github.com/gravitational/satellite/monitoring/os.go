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

import "github.com/gravitational/trace"

// NewOSChecker returns a new checker to verify OS distribution
// against the list of supported releases.
//
// The checker only supports Linux.
func NewOSChecker(releases ...OSRelease) noopChecker {
	return noopChecker{}
}

// OSRelease describes an OS distribution.
// It only supports Linux.
type OSRelease struct {
	// ID identifies the distributor: ubuntu, redhat/centos, etc.
	ID string
	// VersionID is the release version i.e. 16.04 for Ubuntu
	VersionID string
	// Like specifies the list of root OS distributions this
	// distribution is a descendant of
	Like []string
}

// GetOSRelease deteremines the OS distribution release information.
//
// It only supports Linux.
func GetOSRelease() (*OSRelease, error) {
	return nil, trace.BadParameter("not implemented")
}
