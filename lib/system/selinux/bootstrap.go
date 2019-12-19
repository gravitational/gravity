package selinux

import (
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	liblog "github.com/gravitational/gravity/lib/log"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/opencontainers/selinux/go-selinux"
	log "github.com/sirupsen/logrus"
)

// Bootstrap executes the bootstrap script for the specified directory
func Bootstrap(config BootstrapConfig) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := utils.WithTempDir(installDefaultPolicy, "policy"); err != nil {
		return trace.Wrap(err)
	}
	return importLocalChanges(config)
}

// GravityInstallerProcessContext specifies the SELinux process context template
// for the installer process.
// Will be used to verify the context of the running process and make bootstrapping
// decisions
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

// WriteBootstrapScript creates the bootstrap script using the specified writer
func WriteBootstrapScript(w io.Writer, config BootstrapConfig) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	var values = struct {
		Path       string
		PortRanges portRanges
	}{
		Path: config.Path,
		PortRanges: portRanges{
			Installer:  schema.DefaultPortRanges.Installer,
			Kubernetes: schema.DefaultPortRanges.Kubernetes,
			Generic:    schema.DefaultPortRanges.Generic,
			VxlanPort:  config.VxlanPort,
		},
	}
	return trace.Wrap(bootstrapScript.Execute(w, values))
}

// BootstrapConfig defines the SELinux bootstrap configuration
type BootstrapConfig struct {
	// Path specifies the location of the installer files
	// FIXME: remove gravity_installer_home_t and use user_home_t with
	// custom type transitions
	Path string
	// VxlanPort specifies the custom vxlan port.
	// Defaults to defaults.VxlanPort
	VxlanPort int
}

func (r *BootstrapConfig) checkAndSetDefaults() error {
	if r.Path == "" {
		r.Path = utils.Exe.WorkingDir
	}
	if r.VxlanPort == 0 {
		r.VxlanPort = defaults.VxlanPort
	}
	return nil
}

func installDefaultPolicy(dir string) error {
	for _, policy := range []string{"container.pp.bz2", "gravity.pp.bz2"} {
		f, err := Policy.Open(policy)
		if err != nil {
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

func importLocalChanges(config BootstrapConfig) error {
	var buf bytes.Buffer
	if err := WriteBootstrapScript(&buf, config); err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command("semanage", "import")
	logger := liblog.New(log.WithField(trace.Component, "system:selinux"))
	w := logger.Writer()
	defer w.Close()
	cmd.Stdin = &buf
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err)
	}
	cmd = exec.Command("restorecon", "-Rvi",
		config.Path,
		defaults.GravitySystemLogPath,
		defaults.GravityUserLogPath,
	)
	cmd.Stdout = w
	cmd.Stderr = w
	log.WithField("cmd", cmd.Args).Info("Restore file contexts.")
	return trace.Wrap(cmd.Run())
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

type portRanges struct {
	Installer  []schema.PortRange
	Kubernetes []schema.PortRange
	Generic    []schema.PortRange
	VxlanPort  int
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
port -a -t gravity_vxlan_port_t -r 's0' -p udp {{.PortRanges.VxlanPort}}

fcontext -a -f f -t gravity_installer_exec_t -r 's0' '{{.Path}}/gravity'
fcontext -a -f f -t gravity_log_t -r 's0' '{{.Path}}/gravity-(install|system)\.log'
fcontext -a -f a -t gravity_home_t -r 's0' '{{.Path}}/.gravity(/.*)?'
fcontext -a -f f -t gravity_unit_file_t -r 's0' '{{.Path}}/.gravity/gravity-(installer|agent)\.service'
fcontext -a -f s -t gravity_var_run_t -r 's0' '{{.Path}}/.gravity/installer\.sock'
`))

var unloadScript = template.Must(template.New("selinux-unload").Parse(`
{{range .PortRanges.Installer}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end -}}
{{range .PortRanges.Kubernetes}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end -}}
{{range .PortRanges.Generic}}
port -d -p {{.Protocol}} {{.From}}-{{.To}}{{end}}
port -d -p {{.Protocol}} {{.PortRanges.VxlanPort}}

fcontext -d -f f '{{.Path}}/gravity'
fcontext -d -f f '{{.Path}}/gravity-(install|system)\.log'
fcontext -d -f a '{{.Path}}/.gravity(/.*)?'
fcontext -d -f f '{{.Path}}/.gravity/gravity-(installer|agent)\.service'
fcontext -d -f s '{{.Path}}/.gravity/installer\.sock'
`))
