/*
Copyright 2016 Gravitational, Inc.

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
	"bytes"
	"context"
	"os/exec"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"

	"github.com/coreos/go-systemd/v22/dbus"
)

// NewSystemdChecker returns a new checker that reports status
// of systemd units
func NewSystemdChecker() systemdChecker {
	return systemdChecker{}
}

// SystemStatusType describes the unit status
type SystemStatusType string

const (
	// SystemStatusRunning is the status of an active unit
	SystemStatusRunning SystemStatusType = "running"
	// SystemStatusDegraded is the status of a failed unit
	SystemStatusDegraded = "degraded"
	// SystemStatusLoading indicates the unit in initializing or starting state
	SystemStatusLoading = "loading"
	// SystemStatusStopped indicates that the unit is stopped
	SystemStatusStopped = "stopped"
	// SystemStatusUnknown is the value of the unit state when it is unknown
	SystemStatusUnknown = ""
)

// Name returns the name of this checker.
// Implements health.Checker
func (r systemdChecker) Name() string { return "systemd" }

// Check evaluates the status of systemd.
// Implements health.Checker
func (r systemdChecker) Check(ctx context.Context, reporter health.Reporter) {
	systemStatus, err := IsSystemRunning()
	if err != nil {
		reason := "failed to query system health"
		reporter.Add(NewProbeFromErr(r.Name(), reason, trace.Wrap(err)))
	}

	conditions, err := systemdStatus()
	if err != nil {
		reporter.Add(NewProbeFromErr(r.Name(), "failed to check systemd status", trace.Wrap(err)))
	}

	if len(conditions) > 0 && SystemStatusType(systemStatus) == SystemStatusRunning {
		systemStatus = SystemStatusDegraded
	}

	for _, condition := range conditions {
		reporter.Add(&pb.Probe{
			Checker: r.Name(),
			Detail:  condition.name,
			Status:  condition.status,
			Error:   condition.err.Error(),
		})
	}
}

func systemdStatus() ([]serviceStatus, error) {
	conn, err := dbus.New()
	if err != nil {
		return nil, trace.Wrap(err, "failed to connect to dbus")
	}
	defer conn.Close()

	var units []dbus.UnitStatus
	units, err = conn.ListUnits()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query systemd units")
	}

	var conditions []serviceStatus
	for _, unit := range units {
		if unit.ActiveState == activeStateFailed || unit.LoadState == loadStateError {
			conditions = append(conditions, serviceStatus{
				name:   unit.Name,
				status: pb.Probe_Failed,
				err:    trace.Errorf("%s", unit.SubState),
			})
		}
	}

	return conditions, nil
}

// IsSystemRunning return the state of systemd
func IsSystemRunning() (SystemStatusType, error) {
	output, err := exec.Command(systemStatusCmd[0], systemStatusCmd[1:]...).CombinedOutput()
	if err != nil && !isExitError(err) {
		return SystemStatusUnknown, trace.Wrap(err)
	}

	var status SystemStatusType
	switch string(bytes.TrimSpace(output)) {
	case "initializing", "starting":
		status = SystemStatusLoading
	case "stopping", "offline":
		status = SystemStatusStopped
	case "degraded":
		status = SystemStatusDegraded
	case "running":
		status = SystemStatusRunning
	default:
		status = SystemStatusUnknown
	}
	return status, nil
}

func isExitError(err error) bool {
	if _, ok := err.(*exec.ExitError); ok {
		return true
	}
	return false
}

// SystemChecker is a health checker for services managed by systemd/monit.
type systemdChecker struct{}

type serviceStatus struct {
	name   string
	status pb.Probe_Type
	err    error
}

var systemStatusCmd = []string{"/bin/systemctl", "is-system-running"}

type loadState string

const (
	loadStateLoaded   loadState = "loaded"
	loadStateError              = "error"
	loadStateMasked             = "masked"
	loadStateNotFound           = "not-found"
)

type activeState string

const (
	activeStateActive       activeState = "active"
	activeStateReloading                = "reloading"
	activeStateInactive                 = "inactive"
	activeStateFailed                   = "failed"
	activeStateActivating               = "activating"
	activeStateDeactivating             = "deactivating"
)
