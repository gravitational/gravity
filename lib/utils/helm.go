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

package utils

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

// CheckHelm checks that helm binary is properly installed
func CheckHelm() error {
	_, err := exec.LookPath("helm")
	if err != nil {
		return trace.BadParameter(
			"helm binary is not found or not executable (%q), check https://docs.helm.sh/using_helm/#installing-helm for details", err)
	}
	buf := &bytes.Buffer{}
	err = Exec(exec.Command("helm", "plugin", "list"), buf)
	if err != nil {
		return trace.BadParameter("failed to run 'helm plugin list' command: %v", err)
	}

	if !strings.Contains(buf.String(), "template") {
		return trace.BadParameter("helm template plugin is not found in installed plugins, install using 'helm plugin install https://github.com/technosophos/helm-template'")
	}
	return nil
}
