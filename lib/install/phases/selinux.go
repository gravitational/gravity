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

package phases

import (
	"bytes"
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/system/selinux"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewSELinux returns executor that configures SELinux
func NewSELinux(p fsm.ExecutorParams, operator ops.Operator, apps app.Applications) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      fsm.OperationKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	app, err := apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	profile, err := app.Manifest.NodeProfiles.ByName(p.Phase.Data.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ports, err := getPorts(*profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	paths, err := getPaths(*profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := operator.GetSiteOperation(fsm.OperationKey(p.Plan))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vxlanPort := operation.InstallExpand.Vars.OnPrem.VxlanPort
	config := selinux.UpdateConfig{
		Generic:   ports,
		Paths:     paths,
		VxlanPort: &vxlanPort,
	}
	return &seLinux{
		FieldLogger:    logger,
		ExecutorParams: p,
		config:         config,
	}, nil
}

// Execute creates system Kubernetes resources.
func (r *seLinux) Execute(ctx context.Context) error {
	r.Progress.NextStep("Configuring SELinux")
	r.Info("Configuring SELinux.")
	return r.config.Update(ctx)
}

// Rollback deletes created system Kubernetes resources.
func (r *seLinux) Rollback(ctx context.Context) error {
	r.Info("Rolling back SELinux configuration.")
	if err := r.config.Undo(ctx); err != nil {
		return trace.Wrap(err)
	}
	return r.applyFileContexts(ctx)
}

// PreCheck is no-op for this phase.
func (r *seLinux) PreCheck(context.Context) error { return nil }

// PostCheck is no-op for this phase.
func (r *seLinux) PostCheck(context.Context) error { return nil }

// seLinux is executor that creates system Kubernetes resources.
type seLinux struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams contains common executor parameters.
	fsm.ExecutorParams
	config selinux.UpdateConfig
}

func (r *seLinux) applyFileContexts(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	// Set file/directory labels as defined by the local changes
	var out bytes.Buffer
	if err := selinux.ApplyFileContexts(ctx, &out, paths...); err != nil {
		return trace.Wrap(err, "failed to restore file contexts: %s", out.String())
	}
	r.WithField("output", out.String()).Info("Restore file contexts.")
	return nil
}

func getPorts(profile schema.NodeProfile) (ports []schema.PortRange, err error) {
	// TODO: validate manifest earlier incl. the port requirements
	tcp, udp, err := profile.Ports()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, port := range tcp {
		ports = append(ports, schema.PortRange{
			From:     uint64(port),
			To:       uint64(port),
			Protocol: "tcp",
		})
	}
	for _, port := range udp {
		ports = append(ports, schema.PortRange{
			From:     uint64(port),
			To:       uint64(port),
			Protocol: "udp",
		})
	}
	return ports, nil
}

func getPaths(profile schema.NodeProfile) (paths []selinux.Path, err error) {
	for _, volume := range profile.Requirements.Volumes {
		if volume.Label == "" {
			volume.Label = defaults.ContainerFileLabel
		}
		if !selinux.ShouldLabelVolume(volume.Label) {
			continue
		}
		paths = append(paths, selinux.Path{
			Path:  volume.Path,
			Label: volume.Label,
		})
	}
	return paths, nil
}

func shouldLabel(label string) bool {
	// TODO: come up with a better way to avoid labeling
	return label == "<<none>>"
}
