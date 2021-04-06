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
	"encoding/json"
	"sort"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/monitoring"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Fix takes a list of failed probes and attempts to fix some of them
func Fix(ctx context.Context, probes []*agentpb.Probe, progress utils.Progress) (fixed, unfixed []*agentpb.Probe) {
	// reorder the probes so "kernel module" ones go before "sysctl parameter"
	// ones because some kernel parameters cannot be set unless a certain
	// module is loaded, so they have to be fixed in order
	sort.Slice(probes, func(i, j int) bool {
		return probes[i].Checker == monitoring.KernelModuleCheckerID
	})
	for _, probe := range probes {
		// we should only have gotten failed probes here but in case we got
		// something else, skip it
		if probe.Status != agentpb.Probe_Failed {
			continue
		}
		if err := fixProbe(ctx, probe, progress); err != nil {
			if !trace.IsNotImplemented(err) {
				logrus.Debugf("Failed to auto-fix probe %#v: %v", *probe, err)
			}
			unfixed = append(unfixed, probe)
		} else {
			fixed = append(fixed, probe)
		}
	}
	return fixed, unfixed
}

// AutoloadModules generates a systemd modules-load.d file for all kernel modules required by gravity.
// modules-load.d is smart enough to skip builtin modules, so we just add all modules we require to the list,
// even if they are added or already running for other reasons.
func AutoloadModules(ctx context.Context, modules []monitoring.ModuleRequest, progress utils.Progress) {
	modList := make([]string, 0, len(modules))
	for _, module := range modules {
		modList = append(modList, module.Name)
		if err := utils.EnsureLineInFile(defaults.ModulesPath, module.Name); err != nil && !trace.IsAlreadyExists(err) {
			progress.PrintWarn(err, "Could not set up kernel module %v to load on boot", module)
		}
		for _, altName := range module.Names {
			if err := utils.EnsureLineInFile(defaults.ModulesPath, altName); err != nil && !trace.IsAlreadyExists(err) {
				progress.PrintWarn(err, "Could not set up kernel module %v to load on boot", altName)
			}
		}
	}
	progress.PrintInfo("Enabled kernel module loading on boot: %v", strings.Join(modList, ","))
}

// GetFixable returns a list of failed probes that can be attempted to auto-fix
func GetFixable(probes []*agentpb.Probe) (failed, fixable []*agentpb.Probe) {
	for _, probe := range probes {
		// we should only have gotten failed probes here but in case we got
		// something else, skip it
		if probe.Status == agentpb.Probe_Failed {
			switch probe.Checker {
			case monitoring.KernelModuleCheckerID, monitoring.IPForwardCheckerID, monitoring.NetfilterCheckerID, monitoring.MountsCheckerID:
				fixable = append(fixable, probe)
			default:
				failed = append(failed, probe)
			}
		}
	}
	return failed, fixable
}

// fixProbe attempts to fix the provided failed probe
func fixProbe(ctx context.Context, probe *agentpb.Probe, progress utils.Progress) error {
	switch probe.Checker {
	case monitoring.KernelModuleCheckerID:
		var data monitoring.KernelModuleCheckerData
		if err := json.Unmarshal(probe.CheckerData, &data); err != nil {
			return trace.Wrap(err)
		}
		if data.Module.Name == "" {
			return trace.BadParameter("empty probe data: %#v", data)
		}
		if err := modprobe(ctx, data.Module.Name, data.Module.Names, progress); err != nil {
			return trace.Wrap(err)
		}
	case monitoring.IPForwardCheckerID, monitoring.NetfilterCheckerID, monitoring.MountsCheckerID:
		var data monitoring.SysctlCheckerData
		if err := json.Unmarshal(probe.CheckerData, &data); err != nil {
			return trace.Wrap(err)
		}
		if data.ParameterName == "" || data.ParameterValue == "" {
			return trace.BadParameter("empty probe data: %#v", data)
		}
		if err := setSysctlParameter(ctx, data.ParameterName, data.ParameterValue, progress); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.NotImplemented("probe %v can't be auto-fixed", probe.Checker)
	}
	return nil
}
