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

package report

import (
	"fmt"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
)

// ResourceCollectors returns gravity resource collectors.
func ResourceCollectors() Collectors {
	// Collect select gravity resources. More information on supported gravity
	// resources found at https://gravitational.com/gravity/docs/config/
	resources := []string{
		storage.KindClusterConfiguration,
		storage.KindRuntimeEnvironment,
		storage.KindAuthGateway,
		storage.KindSMTPConfig,
		storage.KindAlertTarget,
		storage.KindAlert,
		storage.KindLogForwarder,
	}

	collectors := make(Collectors, len(resources))
	for i, resource := range resources {
		collectors[i] = Cmd(fmt.Sprintf("%s.yaml", resource), gravityResourceYAML(resource)...)
	}
	return collectors
}

// gravityResourceYAML returns the gravity command to output the specified
// resource in YAML format.
func gravityResourceYAML(resource string) []string {
	return utils.Self("resource", "get", "--format", "yaml", resource)
}
