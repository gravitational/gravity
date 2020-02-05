package selinux

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// Bootstrap executes the bootstrap script for the specified directory
func Bootstrap(config BootstrapConfig) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	metadata, err := monitoring.GetOSRelease()
	if err != nil {
		return trace.Wrap(err)
	}
	b := newBootstrapper(config, *metadata)
	err = utils.WithTempDir(func(dir string) error {
		return b.installDefaultPolicy(dir)
	}, "policy")
	if err != nil {
		if trace.IsNotFound(err) {
			log.WithError(err).Warn("OS distribution is not supported, SELinux will not be turned on.")
			return nil
		}
		return trace.Wrap(err)
	}
	return b.importLocalChanges()
}

// Unload removes the policy modules and local modifications
func Unload(config BootstrapConfig) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	metadata, err := monitoring.GetOSRelease()
	if err != nil {
		return trace.Wrap(err)
	}
	b := newBootstrapper(config, *metadata)
	return b.removeLocalChanges()
	// TODO: remove the policy module.
	// Note, this is only possible whebn there're no more gravity-labeled
	// files/directories.
}

// Patch executes the patch script with the underlying configuration
func (r PatchConfig) Patch() error {
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	port, err := getLocalPortChangeForVxlan()
	if err != nil {
		return trace.Wrap(err)
	}
	if port.Range == strconv.Itoa(r.VxlanPort) {
		// Nothing to do
		log.Info("Vxlan port is default: nothing to do.")
		return nil
	}
	var buf bytes.Buffer
	w := logger.Writer()
	defer w.Close()
	if err := r.writeTo(io.MultiWriter(&buf, w)); err != nil {
		return trace.Wrap(err)
	}
	return importLocalChangesFromReader(&buf)
}

// writeTo writes the patch script to the specified writer
func (r PatchConfig) writeTo(w io.Writer) error {
	var values = struct {
		DefaultVxlanPort, VxlanPort int
	}{
		DefaultVxlanPort: defaults.VxlanPort,
		VxlanPort:        r.VxlanPort,
	}
	return trace.Wrap(patchScript.Execute(w, values))
}

// GravityInstallerProcessContext specifies the expected SELinux process domain.
// During bootstrapping, after the policy has been loaded, the process is
// configured to start under a new domain (if not already) and restarted.
var GravityInstallerProcessContext = MustNewContext("system_u:system_r:gravity_installer_t:s0")

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
	metadata, err := monitoring.GetOSRelease()
	if err != nil {
		return trace.Wrap(err)
	}
	return newBootstrapper(config, *metadata).writeBootstrapScript(w)
}

// BootstrapConfig defines the SELinux bootstrap configuration
type BootstrapConfig struct {
	// Path specifies the location of the installer files
	Path string
	// StateDir specifies the custom system state directory.
	// Will be used only if specified
	StateDir string
	// VxlanPort specifies the custom vxlan port.
	// If left unspecified (nil), will not be configured
	VxlanPort  *int
	portRanges *portRanges
}

func (r *BootstrapConfig) checkAndSetDefaults() error {
	if r.Path == "" {
		r.Path = utils.Exe.WorkingDir
	}
	if r.portRanges == nil {
		r.portRanges = &portRanges{
			Installer:  libschema.DefaultPortRanges.Installer,
			Kubernetes: libschema.DefaultPortRanges.Kubernetes,
			Generic:    libschema.DefaultPortRanges.Generic,
			VxlanPort:  r.VxlanPort,
		}
	}
	return nil
}

func (r *BootstrapConfig) isCustomStateDir() bool {
	return r.StateDir != "" && r.StateDir != defaults.GravityDir
}

// PatchConfig describes the configuration to update parts of the SELinux configuration
type PatchConfig struct {
	// VxlanPort specifies the custom vxlan port.
	VxlanPort int
}

func newBootstrapper(config BootstrapConfig, metadata monitoring.OSRelease) *bootstrapper {
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	return &bootstrapper{
		config:           config,
		metadata:         metadata,
		logger:           logger,
		policyFileReader: withPolicy(Policy),
	}
}

type bootstrapper struct {
	config           BootstrapConfig
	metadata         monitoring.OSRelease
	logger           liblog.Logger
	policyFileReader policyFileReader
}

func (r *bootstrapper) installDefaultPolicy(dir string) error {
	for _, policy := range []string{"container.pp.bz2", "gravity.pp.bz2"} {
		f, err := r.openFile(policy)
		if err != nil {
			if os.IsNotExist(err) {
				return trace.NotFound("no SELinux policy for the specified OS distribution %v",
					r.metadata.ID).AddField("os", r.metadata.ID)
			}
			return trace.Wrap(err)
		}
		defer f.Close()
		path := filepath.Join(dir, policy)
		err = installPolicyFile(path, f)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (r *bootstrapper) importLocalChanges() error {
	var buf bytes.Buffer
	w := r.logger.Writer()
	defer w.Close()
	if err := r.writeBootstrapScript(io.MultiWriter(&buf, w)); err != nil {
		return trace.Wrap(err)
	}
	err := importLocalChangesFromReader(&buf)
	if err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command("restorecon", "-Rvi",
		r.config.Path,
		defaults.GravitySystemLogPath,
		defaults.GravityUserLogPath,
	)
	cmd.Stdout = w
	cmd.Stderr = w
	r.logger.WithField("cmd", cmd.Args).Info("Restore file contexts.")
	return trace.Wrap(cmd.Run())
}

func (r *bootstrapper) removeLocalChanges() error {
	var buf bytes.Buffer
	if err := r.writeUnloadScript(&buf); err != nil {
		return trace.Wrap(err)
	}
	return importLocalChangesFromReader(&buf)
}

func (r *bootstrapper) writeBootstrapScript(w io.Writer) error {
	var values = struct {
		Path       string
		PortRanges portRanges
	}{
		Path: r.config.Path,
	}
	if r.config.portRanges != nil {
		values.PortRanges = *r.config.portRanges
	}
	values.PortRanges.VxlanPort = r.config.VxlanPort
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
		StateDir: r.config.StateDir,
		Path:     r.config.Path,
	}
	if r.config.portRanges != nil {
		values.PortRanges = *r.config.portRanges
		values.PortRanges.VxlanPort = r.config.VxlanPort
		if values.PortRanges.VxlanPort == nil {
			values.PortRanges.VxlanPort = utils.IntPtr(defaults.VxlanPort)
		}
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
	f, err := r.policyFileReader.OpenFile(filepath.Join(mapDistro(r.metadata.ID), name))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

func renderFcontext(w io.Writer, stateDir string, fcontextTemplate io.Reader, renderer commandRenderer) error {
	b, err := ioutil.ReadAll(fcontextTemplate)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	tmpl, err := template.New("fcontext").Parse(string(b))
	if err != nil {
		return trace.Wrap(err)
	}
	var buf bytes.Buffer
	var values = struct {
		StateDir string
	}{
		StateDir: stateDir,
	}
	if err := tmpl.Execute(&buf, values); err != nil {
		return trace.Wrap(err)
	}
	items, err := schema.ParseFcontextFile(&buf)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, item := range items {
		if _, err := fmt.Fprint(w, renderer(item), "\n"); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func withPolicy(policy http.FileSystem) policyFileReader {
	return policyFileReaderFunc(func(path string) (io.ReadCloser, error) {
		f, err := policy.Open(path)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		return f, nil
	})
}

type policyFileReader interface {
	OpenFile(name string) (io.ReadCloser, error)
}

func (r policyFileReaderFunc) OpenFile(name string) (io.ReadCloser, error) {
	return r(name)
}

type policyFileReaderFunc func(name string) (io.ReadCloser, error)

type commandRenderer func(schema.FcontextFileItem) string

func installPolicyFile(path string, r io.Reader) error {
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	logger.WithField("path", path).Info("Install policy file.")
	if err := utils.CopyReaderWithPerms(path, r, defaults.SharedReadMask); err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command("semodule", "--install", path)
	w := logger.Writer()
	defer w.Close()
	cmd.Stdout = w
	cmd.Stderr = w
	return trace.Wrap(cmd.Run())
}

func getLocalPortChangeForVxlan() (*schema.PortCommand, error) {
	const gravityVxlanPortType = "gravity_vxlan_port_t"
	cmd := exec.Command("semanage", "port", "--extract")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, trace.Wrap(err, string(out))
	}
	localPorts, err := schema.GetLocalPortChangesFromReader(bytes.NewReader(out))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, port := range localPorts {
		if port.Type == gravityVxlanPortType {
			return &port, nil
		}
	}
	return nil, trace.NotFound("no override for vxlan port")
}

func importLocalChangesFromReader(r io.Reader) error {
	cmd := exec.Command("semanage", "import")
	logger := liblog.New(log.WithField(trace.Component, "system:selinux"))
	w := logger.Writer()
	defer w.Close()
	cmd.Stdin = r
	cmd.Stdout = w
	cmd.Stderr = w
	return trace.Wrap(cmd.Run())
}

func removePolicy() error {
	// Leave the container package intact as we might not be
	// the only client
	return removePolicyByName("gravity")
}

func removePolicyByName(module string) error {
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	logger.WithField("module", module).Info("Remove policy module.")
	cmd := exec.Command("semodule", "--remove", module)
	w := logger.Writer()
	defer w.Close()
	cmd.Stdout = w
	cmd.Stderr = w
	return trace.Wrap(cmd.Run())
}

type portRanges struct {
	Installer  []libschema.PortRange
	Kubernetes []libschema.PortRange
	Generic    []libschema.PortRange
	VxlanPort  *int
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
{{if .PortRanges.VxlanPort}}port -a -t gravity_vxlan_port_t -r 's0' -p udp {{.PortRanges.VxlanPort}}{{end}}
fcontext -a -f f -t gravity_installer_exec_t -r 's0' '{{.Path}}/gravity'
fcontext -a -f f -t gravity_log_t -r 's0' '{{.Path}}/gravity-(install|system)\.log'
fcontext -a -f a -t gravity_home_t -r 's0' '{{.Path}}/.gravity(/.*)?'
`))

var unloadScript = template.Must(template.New("selinux-unload").Parse(`
port -D
fcontext -D
`))

// TODO: reflect the actual state of local customizations (incl. custom vxlan port)
var unloadScript0 = template.Must(template.New("selinux-unload0").Parse(`
{{range .PortRanges.Installer}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
{{- range .PortRanges.Kubernetes}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
{{- range .PortRanges.Generic}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
port -d -p udp {{.PortRanges.VxlanPort}}

fcontext -d -f f '{{.Path}}/gravity'
fcontext -d -f f '{{.Path}}/gravity-(install|system)\.log'
fcontext -d -f a '{{.Path}}/.gravity(/.*)?'
`))

var patchScript = template.Must(template.New("selinux-patch").Parse(`
port -d -p udp {{.DefaultVxlanPort}}
port -a -t gravity_vxlan_port_t -r 's0' -p udp {{.VxlanPort}}
`))

func mapDistro(distroID string) string {
	switch distroID {
	case "centos", "rhel":
		return "centos"
	default:
		return distroID
	}
}
