package autofix

import (
	"context"
	"encoding/json"
	"sort"

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
			logrus.Debugf("Failed to auto-fix probe %#v: %v", *probe, err)
			unfixed = append(unfixed, probe)
		} else {
			fixed = append(fixed, probe)
		}
	}
	return fixed, unfixed
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
		if err := enableKernelModule(ctx, data.Module.Name, data.Module.Names, progress); err != nil {
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
