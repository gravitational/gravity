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

package systeminfo

import (
	"fmt"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/satellite/monitoring"
	"github.com/gravitational/trace"
)

// RedHat identifies a RedHat Enterprise Linux system or one of its descent
const RedHat = "rhel"

// OSInfo obtains identification information for the host operating system
func OSInfo() (info *OS, err error) {
	metadata, err := monitoring.GetOSRelease()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &OS{
		ID:      metadata.ID,
		Version: metadata.VersionID,
		Like:    metadata.Like,
	}, nil
}

// OS aliases operating system info
type OS storage.OSInfo

// IsRedHat determines if this info refers to a RedHat system or one of its descent
func (r OS) IsRedHat() bool {
	return r.ID == RedHat || utils.StringInSlice(r.Like, RedHat)
}

// Name returns a name/version for this OS info, e.g. "centos 7.1"
func (r OS) Name() string {
	return fmt.Sprintf("%v %v", r.ID, r.Version)
}
