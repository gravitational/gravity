package selinux

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/Azure/go-autorest/logger"
	"github.com/gravitational/gravity/lib/defaults"
	liblog "github.com/gravitational/gravity/lib/log"
	libschema "github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/system/selinux/schema"
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
	b := newBootstrapper(config, metadata)
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
	// FIXME: rewrite using bootstrapper
	// FIXME: unload script needs to use another API - not writeBootstrapScript
	if err := removeLocalChanges(config); err != nil {
		return trace.Wrap(err)
	}
	return removePolicy()
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
	// FIXME
	var metadata monitoring.OSRelease
	return newBootstrapper(config, metadata).writeBootstrapScript(w)
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
	VxlanPort *int
	// Force forces the update of the policy
	Force bool
}

func (r *BootstrapConfig) checkAndSetDefaults() error {
	if r.Path == "" {
		r.Path = utils.Exe.WorkingDir
	}
	return nil
}

func (r *BootstrapConfig) isCustomStateDir() bool {
	return r.StateDir != defaults.GravityDir
}

// PatchConfig describes the configuration to update parts of the SELinux configuration
type PatchConfig struct {
	// VxlanPort specifies the custom vxlan port.
	VxlanPort int
}

func newBootstrapper(config BootstrapConfig, metadata monitoring.OSRelease) *bootstrapper {
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	return &bootstrapper{
		config:   config,
		metadata: metadata,
		logger:   logger,
	}
}

type bootstrapper struct {
	config   BootstrapConfig
	metadata monitoring.OSRelease
	logger   liblog.Logger
}

func (r *bootstrapper) writeBootstrapScript(w io.Writer) error {
	var values = struct {
		Path       string
		PortRanges portRanges
	}{
		Path: r.config.Path,
		PortRanges: portRanges{
			Installer:  libschema.DefaultPortRanges.Installer,
			Kubernetes: libschema.DefaultPortRanges.Kubernetes,
			Generic:    libschema.DefaultPortRanges.Generic,
			VxlanPort:  r.config.VxlanPort,
		},
	}
	if err := trace.Wrap(bootstrapScript.Execute(w, values)); err != nil {
		return trace.Wrap(err)
	}
	if !r.config.isCustomStateDir() {
		return nil
	}
	f, err := r.openFile("gravity.statedir.fc.template")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	var fcontext bytes.Buffer
	return renderFcontext(&fcontext, r.config.StateDir, f)
}

func (r *bootstrapper) installDefaultPolicy(dir string) error {
	for _, policy := range []string{"container.pp.bz2", "gravity.pp.bz2"} {
		// f, err := r.openFile(filepath.Join(mapDistro(metadata.ID), policy))
		f, err := r.openFile(policy)
		if err != nil {
			if os.IsNotExist(err) {
				return trace.NotFound("no SELinux policy for the specified OS distribution %v",
					r.metadata.ID).AddField("distribution", r.metadata.ID)
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

func (r *bootstrapper) importLocalChanges(config BootstrapConfig) error {
	var buf bytes.Buffer
	w := logger.Writer()
	defer w.Close()
	if err := r.writeBootstrapScript(io.MultiWriter(&buf, w)); err != nil {
		return trace.Wrap(err)
	}
	err := importLocalChangesFromReader(&buf)
	if err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command("restorecon", "-Rvi",
		config.Path,
		defaults.GravitySystemLogPath,
		defaults.GravityUserLogPath,
	)
	cmd.Stdout = w
	cmd.Stderr = w
	log.WithField("cmd", cmd.Args).Info("Restore file contexts.")
	return trace.Wrap(cmd.Run())
}

func renderFcontext(w io.Writer, stateDir string, fcontextTemplate io.Reader) error {
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
		if _, err := io.WriteString(w, item.AsAddCommand()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

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

func removeLocalChanges(config BootstrapConfig) error {
	var buf bytes.Buffer
	if err := writeUnloadScript(&buf, config); err != nil {
		return trace.Wrap(err)
	}
	return importLocalChangesFromReader(&buf)
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

// writeUnloadScript creates the unload script using the specified writer
func writeUnloadScript(w io.Writer, config BootstrapConfig) error {
	var values = struct {
		Path       string
		PortRanges portRanges
	}{
		Path: config.Path,
		PortRanges: portRanges{
			Installer:  libschema.DefaultPortRanges.Installer,
			Kubernetes: libschema.DefaultPortRanges.Kubernetes,
			Generic:    libschema.DefaultPortRanges.Generic,
			VxlanPort:  config.VxlanPort,
		},
	}
	return trace.Wrap(unloadScript.Execute(w, values))
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

{{range .PortRanges.Installer}}
port -a -t gravity_install_port_t -r 's0' -p {{.Protocol}} {{.From}}-{{.To}}{{end -}}
{{range .PortRanges.Kubernetes}}
port -a -t gravity_kubernetes_port_t -r 's0' -p {{.Protocol}} {{.From}}-{{.To}}{{end -}}
{{range .PortRanges.Generic}}
port -a -t gravity_port_t -r 's0' -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
{{if .PortRanges.VxlanPort}}port -a -t gravity_vxlan_port_t -r 's0' -p udp {{.PortRanges.VxlanPort}}{{end}}

fcontext -a -f f -t gravity_installer_exec_t -r 's0' '{{.Path}}/gravity'
fcontext -a -f f -t gravity_log_t -r 's0' '{{.Path}}/gravity-(install|system)\.log'
fcontext -a -f a -t gravity_home_t -r 's0' '{{.Path}}/.gravity(/.*)?'

`))

var unloadScript = template.Must(template.New("selinux-unload").Parse(`
{{range .PortRanges.Installer}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end -}}
{{range .PortRanges.Kubernetes}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end -}}
{{range .PortRanges.Generic}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
{{if .PortRanges.VxlanPort}}port -d -p udp {{.PortRanges.VxlanPort}}{{end}}

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
		return defaultRelease
	default:
		return ""
	}
}

// defaultRelease specifies the default OS distribution name
// that defines the policy files to use if the existing distribution
// is not supported
const defaultRelease = "centos"
