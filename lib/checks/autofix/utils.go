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

package autofix

import (
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// modprobe loads a kernel module by the provided name or, if that fails, by
// trying provided alternative names
func modprobe(ctx context.Context, name string, altNames []string, progress utils.Progress) error {
	var errors []string
	for _, n := range append([]string{name}, altNames...) {
		out, err := utils.RunCommand(ctx, nil, "modprobe", n)
		if err == nil {
			progress.PrintInfo("Auto-loaded kernel module: %v", n)
			return nil
		}
		errors = append(errors, string(out))
	}
	return trace.BadParameter("failed to enable kernel module %v(%v): %s", name, altNames, errors)
}

// setSysctlParameter sets the specified kernel parameter and makes sure it
// persists across reboots
func setSysctlParameter(ctx context.Context, name, value string, progress utils.Progress) error {
	out, err := utils.RunCommand(ctx, nil, "sysctl", "-w", fmt.Sprintf("%v=%v", name, value))
	if err != nil {
		return trace.Wrap(err, "failed to set kernel parameter %v=%v: %s", name, value, out)
	}
	progress.PrintInfo("Auto-set kernel parameter: %v=%v", name, value)
	if err := utils.EnsureLineInFile(defaults.SysctlPath, fmt.Sprintf("%v=%v", name, value)); err != nil && !trace.IsAlreadyExists(err) {
		progress.PrintWarn(err, "Could not set up kernel parameter %v=%v to persist across reboots", name, value)
	}
	return nil
}
