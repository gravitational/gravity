// +build !linux

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

package system

import "github.com/gravitational/trace"

// DropCapabilitiesForJournalExport drops capabilities except those required
// to export a systemd journal
func DropCapabilitiesForJournalExport() error {
	return trace.NotImplemented("API is not supported")
}

// DropCapabilities drops all capabilities except those specified with keep
// from the current process
func DropCapabilities(keep map[int]struct{}) error {
	return trace.NotImplemented("API is not supported")
}
