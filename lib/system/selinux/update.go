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

package selinux

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strconv"
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	liblog "github.com/gravitational/gravity/lib/log"
	libschema "github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/system/selinux/internal/schema"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Update updates the SELinux configuration described by this object
// on the node
func (r UpdateConfig) Update(ctx context.Context) error {
	r.setDefaults()
	var buf bytes.Buffer
	w := r.Logger.Writer()
	defer w.Close()
	if err := r.write(ctx, io.MultiWriter(w, &buf)); err != nil {
		return trace.Wrap(err)
	}
	return importLocalChangesFromReader(ctx, &buf)
}

// Undo undoes the local changes described by this configuration
func (r UpdateConfig) Undo(ctx context.Context) error {
	r.setDefaults()
	var buf bytes.Buffer
	w := r.Logger.Writer()
	defer w.Close()
	if err := r.writeUndo(io.MultiWriter(&buf, w)); err != nil {
		return trace.Wrap(err)
	}
	return importLocalChangesFromReader(ctx, &buf)
}

// UpdateConfig describes the additional local configuration changes
type UpdateConfig struct {
	liblog.Logger
	// Generic lists additional port configuration
	Generic []libschema.PortRange
	// VxlanPort optionally specifies the new vxlan port.
	// If unspecified, will not be updated.
	VxlanPort *int
	// Paths optionally lists additional paths to add file contexts for
	Paths Paths
	vxlanPortGetter
}

func (r *UpdateConfig) setDefaults() {
	if r.Logger == nil {
		r.Logger = liblog.New(log.WithField(trace.Component, "selinux"))
	}
	if r.vxlanPortGetter == nil {
		r.vxlanPortGetter = getLocalPortChangeForVxlan
	}
}

func (r UpdateConfig) write(ctx context.Context, w io.Writer) error {
	var existingVxlanPort string
	if r.VxlanPort != nil {
		var err error
		existingVxlanPort, err = r.vxlanPortGetter(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if existingVxlanPort == strconv.Itoa(*r.VxlanPort) {
			// No need to update vxlan port
			r.VxlanPort = nil
			existingVxlanPort = ""
		}
	}
	var values = struct {
		Generic           []libschema.PortRange
		ExistingVxlanPort string
		VxlanPort         *int
		Paths             Paths
	}{
		ExistingVxlanPort: existingVxlanPort,
		VxlanPort:         r.VxlanPort,
		Generic:           r.Generic,
		Paths:             r.Paths,
	}
	return updateScript.Execute(w, values)
}

func (r *UpdateConfig) writeUndo(w io.Writer) error {
	var values = struct {
		Generic   []libschema.PortRange
		VxlanPort int
		Paths     Paths
	}{
		VxlanPort: *r.VxlanPort,
		Generic:   r.Generic,
		Paths:     r.Paths,
	}
	return updateUndoScript.Execute(w, values)
}

// Paths returns the paths component of this path list
func (r Paths) Paths() []string {
	paths := make([]string, 0, len(r))
	for _, path := range r {
		paths = append(paths, path.Path)
	}
	return paths
}

// Paths is a list of paths
type Paths []Path

// Path describes a local file context change for a directory
type Path struct {
	// Path specifies the directory path
	Path string
	// Label specifies the SELinux label
	Label string
}

type vxlanPortGetter func(context.Context) (port string, err error)

func getLocalPortChangeForVxlan(ctx context.Context) (port string, err error) {
	const gravityVxlanPortType = "gravity_vxlan_port_t"
	cmd := exec.CommandContext(ctx, "semanage", "port", "--extract")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", trace.Wrap(err, string(out))
	}
	localPorts, err := schema.GetLocalPortChangesFromReader(bytes.NewReader(out))
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, port := range localPorts {
		if port.Type == gravityVxlanPortType {
			return port.Range, nil
		}
	}
	return "", trace.NotFound("no local vxlan port configuration")
}

var updateScript = template.Must(
	template.New("selinux-update").
		Funcs(updateScriptFuncs).
		Parse(`
{{if .ExistingVxlanPort}}port -d -p udp {{.ExistingVxlanPort}}{{end}}
{{if .VxlanPort}}port -a -t gravity_vxlan_port_t -r 's0' -p udp {{.VxlanPort}}{{end}}
{{range .Generic}}
port -a -t gravity_port_t -r 's0' -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
{{range .Paths}}
fcontext -a -f d -t {{labelType .}} -r 's0' '{{.Path}}'{{end -}}
`))

var updateUndoScript = template.Must(
	template.New("selinux-undo-update").
		Funcs(updateScriptFuncs).
		Parse(`
port -d -p udp {{.VxlanPort}}
{{range .Generic}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
{{range .Paths}}
fcontext -d -f d -t {{labelType .}} '{{.Path}}'{{end -}}
`))

var updateScriptFuncs = template.FuncMap{
	"labelType": labelType,
}

func labelType(p Path) (typ string, err error) {
	if p.Label == "" {
		p.Label = defaults.ContainerFileLabel
	}
	ctx, err := schema.NewContext(p.Label)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return ctx.Type, nil
}
