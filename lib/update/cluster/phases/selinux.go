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
	"context"
	"io/ioutil"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/selinux"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewUpdatePhaseSELinux creates a new phase to configure SELinux on a node
func NewUpdatePhaseSELinux(
	p fsm.ExecutorParams,
	operator ops.Operator,
	apps app.Applications,
	logger log.FieldLogger,
) (*updatePhaseSELinux, error) {
	if p.Phase.Data.Package == nil {
		return nil, trace.NotFound("no update application package specified for phase %v", p.Phase)
	}
	if p.Phase.Data.Update == nil || len(p.Phase.Data.Update.Servers) != 1 {
		return nil, trace.BadParameter("server is required for phase %v", p.Phase)
	}
	app, err := apps.GetApp(*p.Phase.Data.InstalledPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateApp, err := apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	server := p.Phase.Data.Update.Servers[0]
	ports, err := diffPorts(app.Manifest, updateApp.Manifest, server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	profile, err := app.Manifest.NodeProfiles.ByName(server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateProfile, err := updateApp.Manifest.NodeProfiles.ByName(server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	paths := diffVolumes(profile.Requirements.Volumes, updateProfile.Requirements.Volumes)
	config := selinux.UpdateConfig{
		Generic: ports,
		Paths:   paths,
	}
	return &updatePhaseSELinux{
		FieldLogger: logger,
		operator:    operator,
		server:      server,
		config:      config,
	}, nil
}

// Execute runs preflight checks
func (p *updatePhaseSELinux) Execute(ctx context.Context) error {
	p.Infof("Configure SELinux on %v.", p.server.Hostname)
	if err := p.config.Update(ctx); err != nil {
		return trace.Wrap(err)
	}
	paths := p.config.Paths.Paths()
	if len(paths) == 0 {
		return nil
	}
	return selinux.ApplyFileContexts(ctx, ioutil.Discard, paths...)
}

// Rollback undos the SELinux configuration changes
func (p *updatePhaseSELinux) Rollback(ctx context.Context) error {
	p.Infof("Roll back SELinux configuration on %v.", p.server.Hostname)
	if err := p.config.Undo(ctx); err != nil {
		return trace.Wrap(err)
	}
	paths := p.config.Paths.Paths()
	if len(paths) == 0 {
		return nil
	}
	return selinux.ApplyFileContexts(ctx, ioutil.Discard, paths...)
}

// PreCheck is no-op for this phase
func (p *updatePhaseSELinux) PreCheck(context.Context) error { return nil }

// PostCheck is no-op for this phase
func (p *updatePhaseSELinux) PostCheck(context.Context) error { return nil }

// updatePhaseSELinux is the update phase to configure SELinux on the nodes
type updatePhaseSELinux struct {
	// FieldLogger specifies the logger used by the executor
	log.FieldLogger
	// operator is the cluster operator service
	operator ops.Operator
	// server is the server where SELinux will be configured
	server storage.UpdateServer
	config selinux.UpdateConfig
}

func diffPorts(old, new schema.Manifest, profileName string) (ports []schema.PortRange, err error) {
	tcp, udp, err := schema.DiffPorts(old, new, profileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ports = make([]schema.PortRange, 0, len(tcp)+len(udp))
	for _, port := range tcp {
		ports = append(ports, schema.PortRange{
			Protocol: "tcp",
			From:     uint64(port),
			To:       uint64(port),
		})
	}
	for _, port := range udp {
		ports = append(ports, schema.PortRange{
			Protocol: "udp",
			From:     uint64(port),
			To:       uint64(port),
		})
	}
	return ports, nil
}

func diffVolumes(old, new []schema.Volume) (paths []selinux.Path) {
	volumes := schema.DiffVolumes(old, new)
	paths = make([]selinux.Path, 0, len(volumes))
	for _, volume := range volumes {
		if !selinux.ShouldLabelVolume(volume.Label) {
			continue
		}
		paths = append(paths, selinux.Path{
			Path:  volume.Path,
			Label: volume.Label,
		})
	}
	return paths
}
