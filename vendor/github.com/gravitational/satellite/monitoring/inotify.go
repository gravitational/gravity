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

package monitoring

import (
	"github.com/gravitational/satellite/agent/health"
)

// NewInotifyChecker creates a new health.Checker that tests if inotify watches
// can be created. This is usually an indication that the system has reached
// fs.inotify.max_user_instances has been exhausted
func NewInotifyChecker() health.Checker {
	return noopChecker{}
}
