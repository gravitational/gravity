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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	liblog "github.com/gravitational/gravity/lib/log"
	libschema "github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/system/selinux/internal/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/satellite/monitoring"
	"github.com/gravitational/trace"
	"github.com/opencontainers/selinux/go-selinux"
	log "github.com/sirupsen/logrus"
)

// Bootstrap configures SELinux on the node.
//
// Bootstrap configuration is comprised of the two policy modules:
// container-selinux policy and gravity-specific policy.
// Also, the process configures the immediately known ports and local file
// contexts for dynamic paths like custom state directory and the installer
// directory.
//
// User-specified port requirements as well custom volumes are configured
// at a later point during the install operation.
func Bootstrap(ctx context.Context, config BootstrapConfig) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	b := newBootstrapper(config)
	err := utils.WithTempDir(func(dir string) error {
		return b.installDefaultPolicy(ctx, dir)
	}, "policy")
	if err != nil {
		return trace.Wrap(err, "failed to install policy module")
	}
	return b.importLocalChanges(ctx)
}

// Unload removes the policy modules and local modifications
func Unload(ctx context.Context, config BootstrapConfig) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	b := newBootstrapper(config)
	return b.removeLocalChanges(ctx)
	// TODO: remove the policy module.
	// Note, this is only possible when there're no more gravity-labeled
	// files/directories.
}

// IsSystemSupported returns true if the system specified with given ID
// is supported
func IsSystemSupported(systemID string) bool {
	switch systemID {
	case "centos", "rhel":
		return true
	default:
		return false
	}
}

// GravityInstallerProcessContext specifies the expected SELinux process domain.
// During bootstrapping, after the policy has been loaded, the process is
// configured to start under a new domain (if not already) and restarted.
var GravityInstallerProcessContext = MustNewContext(defaults.GravityInstallerProcessLabel)

// MustNewContext parses the specified label as SELinux context.
// Panics if label is not a valid SELinux label
func MustNewContext(label string) selinux.Context {
	ctx, err := selinux.NewContext(label)
	if err != nil {
		panic(err)
	}
	return ctx
}

// WriteBootstrapScript writes the bootstrap script to the specified writer
func WriteBootstrapScript(w io.Writer, config BootstrapConfig) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return newBootstrapper(config).writeBootstrapScript(w)
}

// BootstrapConfig defines the SELinux bootstrap configuration
type BootstrapConfig struct {
	// Path specifies the location of the installer files
	Path string
	// StateDir specifies the custom system state directory.
	// Will be used only if specified
	StateDir string
	// OS specifies the OS distribution metadata
	OS         *monitoring.OSRelease
	portRanges portRanges
}

func (r *BootstrapConfig) checkAndSetDefaults() error {
	if r.Path == "" {
		r.Path = utils.Exe.WorkingDir
	}
	if r.OS == nil {
		metadata, err := monitoring.GetOSRelease()
		if err != nil {
			return trace.Wrap(err)
		}
		r.OS = metadata
	}
	if r.portRanges.isEmpty() {
		r.portRanges = portRanges{
			Installer:  libschema.DefaultPortRanges.Installer,
			Kubernetes: libschema.DefaultPortRanges.Kubernetes,
			Generic:    libschema.DefaultPortRanges.Generic,
		}
	}
	return nil
}

// paths returns the list of paths to relabel upon loading
// the policy and the local fcontexts
func (r BootstrapConfig) paths() []string {
	return []string{
		r.Path,
		r.stateDir(),
		defaults.PlanetStateDir,
		defaults.GravityEphemeralDir,
		// TODO: even though it is sort of possible to override the log file paths
		// from command line, the locations are not persisted anywhere so subsequent
		// command will just use the default locations.
		defaults.GravitySystemLogPath,
		defaults.GravityUserLogPath,
	}
}

func (r *BootstrapConfig) stateDir() string {
	if r.StateDir != "" {
		return r.StateDir
	}
	return defaults.GravityDir
}

func (r *BootstrapConfig) isCustomStateDir() bool {
	return r.StateDir != "" && r.StateDir != defaults.GravityDir
}

// Error returns the readable error message
func (r DistributionNotSupportedError) Error() string {
	return fmt.Sprintf("no SELinux policy for OS distribution %v", r.ID)
}

// DistributionNotSupportedError describes an error configuring SELinux
// on an distribution that we do not support SELinux on
type DistributionNotSupportedError struct {
	// ID specifies the OS distribution id
	ID string
}

func newBootstrapper(config BootstrapConfig) *bootstrapper {
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	return &bootstrapper{
		config:           config,
		logger:           logger,
		policyFileReader: policyFileReaderFunc(withPolicy),
	}
}

type bootstrapper struct {
	config           BootstrapConfig
	logger           liblog.Logger
	policyFileReader policyFileReader
}

func (r *bootstrapper) installDefaultPolicy(ctx context.Context, dir string) error {
	for _, policy := range []string{"container.pp.bz2", "gravity.pp.bz2"} {
		f, err := r.openFile(policy)
		if err != nil {
			if os.IsNotExist(err) {
				return DistributionNotSupportedError{ID: r.config.OS.ID}
			}
			return trace.Wrap(err)
		}
		defer f.Close()
		path := filepath.Join(dir, policy)
		err = installPolicyFile(ctx, path, f)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (r *bootstrapper) importLocalChanges(ctx context.Context) error {
	var buf bytes.Buffer
	w := r.logger.Writer()
	defer w.Close()
	if err := r.writeBootstrapScript(io.MultiWriter(&buf, w)); err != nil {
		return trace.Wrap(err)
	}
	err := importLocalChangesFromReader(ctx, &buf)
	if err != nil {
		return trace.Wrap(err)
	}
	paths := r.config.paths()
	if len(paths) == 0 {
		return nil
	}
	r.logger.WithField("paths", paths).Info("Restore file contexts.")
	return ApplyFileContexts(ctx, w, paths...)
}

func (r *bootstrapper) removeLocalChanges(ctx context.Context) error {
	var buf bytes.Buffer
	w := r.logger.Writer()
	defer w.Close()
	if err := r.writeUnloadScript(io.MultiWriter(&buf, w)); err != nil {
		return trace.Wrap(err)
	}
	return importLocalChangesFromReader(ctx, &buf)
}

func (r *bootstrapper) writeBootstrapScript(w io.Writer) error {
	var values = struct {
		Path       string
		PortRanges portRanges
	}{
		Path:       r.config.Path,
		PortRanges: r.config.portRanges,
	}
	if err := bootstrapScript.Execute(w, values); err != nil {
		return trace.Wrap(err)
	}
	return r.renderFcontext(w, schema.FcontextFileItem.AsAddCommand)
}

func (r *bootstrapper) writeUnloadScript(w io.Writer) error {
	var values = struct {
		StateDir   string
		Path       string
		PortRanges portRanges
	}{
		StateDir:   r.config.StateDir,
		Path:       r.config.Path,
		PortRanges: r.config.portRanges,
	}
	if err := unloadScript.Execute(w, values); err != nil {
		return trace.Wrap(err)
	}
	return r.renderFcontext(w, schema.FcontextFileItem.AsRemoveCommand)
}

func (r *bootstrapper) renderFcontext(w io.Writer, renderer commandRenderer) error {
	if !r.config.isCustomStateDir() {
		return nil
	}
	f, err := r.openFile("gravity.statedir.fc.template")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	return renderFcontext(w, r.config.StateDir, f, renderer)
}

func (r *bootstrapper) openFile(name string) (io.ReadCloser, error) {
	f, err := r.policyFileReader.OpenFile(r.pathJoin(name))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

func (r *bootstrapper) pathJoin(elems ...string) string {
	var root string
	switch r.config.OS.ID {
	case "centos", "rhel":
		root = "centos"
	default:
		root = r.config.OS.ID
	}
	return filepath.Join(append([]string{root}, elems...)...)
}

func (r portRanges) isEmpty() bool {
	return len(r.Installer)+len(r.Kubernetes)+len(r.Generic) == 0
}

type portRanges struct {
	Installer  []libschema.PortRange
	Kubernetes []libschema.PortRange
	Generic    []libschema.PortRange
}

// bootstrapScript defines the set of modifications to bootstrap the installer from
// the location specified with .Path as the installer location.
// The commands are used in conjuction with `semanage import/export`
// TODO(dmitri): currently the template drops all local ports/file contexts as the
// corresponding 'semanage' commands fail unconditionally for existing mappings and
// it would require more work to figure out the correct diff to delete
var bootstrapScript = template.Must(template.New("selinux-bootstrap").Parse(`
port -D
fcontext -D

{{- range .PortRanges.Installer}}
port -a -t gravity_install_port_t -r 's0' -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
{{- range .PortRanges.Kubernetes}}
port -a -t gravity_kubernetes_port_t -r 's0' -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
{{- range .PortRanges.Generic}}
port -a -t gravity_port_t -r 's0' -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
fcontext -a -f f -t gravity_installer_exec_t -r 's0' '{{.Path}}/gravity'
fcontext -a -f f -t gravity_log_t -r 's0' '{{.Path}}/gravity-(install|system)\.log'
fcontext -a -f a -t gravity_home_t -r 's0' '{{.Path}}/.gravity(/.*)?'
fcontext -a -f a -t gravity_home_t -r 's0' '{{.Path}}/crashreport.tgz'
`))

var unloadScript = template.Must(template.New("selinux-unload").Parse(`
port -D
fcontext -D
`))
