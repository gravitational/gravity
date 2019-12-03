package selinux

import (
	"bytes"
	"io"
	"os/exec"
	"text/template"

	liblog "github.com/gravitational/gravity/lib/log"

	"github.com/gravitational/trace"
	"github.com/opencontainers/selinux/go-selinux"
	log "github.com/sirupsen/logrus"
)

// Bootstrap executes the bootstrap script for the specified directory
func Bootstrap(workingDir string) error {
	var buf bytes.Buffer
	if err := WriteBootstrapScript(&buf, workingDir); err != nil {
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
	cmd = exec.Command("restorecon", "-Rv", workingDir)
	cmd.Stdout = w
	cmd.Stderr = w
	return trace.Wrap(cmd.Run())
}

// GravityProcessContext specifies the SELinux process context template
var GravityProcessContext = MustNewContext("system_u:system_r:gravity_t:s0")

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
func WriteBootstrapScript(w io.Writer, path string) error {
	var config = struct {
		Path string
	}{
		Path: path,
	}
	return trace.Wrap(bootstrapScript.Execute(w, config))
}

// bootstrapScript defines the set of modifications to bootstrap the installer from
// the location specified with .Path as the installer location.
// The commands are used in conjuction with `semanage import/export`
var bootstrapScript = template.Must(template.New("selinux").Parse(`
port -d -p tcp 61009-61010
port -d -p tcp 61022-61025
port -d -p tcp 61080
port -d -p tcp 7575
port -d -p tcp 3012
port -d -p tcp 3022-3025
port -d -p tcp 3008-3011
port -d -p tcp 3080
port -d -p tcp 32009
port -d -p tcp 2379-2380
port -d -p tcp 6443
port -d -p tcp 7373
port -d -p tcp 7496
port -d -p tcp 10248-10255
port -d -p tcp 8472

port -a -t gravity_install_port_t -r 's0' -p tcp 61009-61010
port -a -t gravity_install_port_t -r 's0' -p tcp 61022-61025
port -a -t gravity_install_port_t -r 's0' -p tcp 61080
port -a -t gravity_agent_port_t -r 's0' -p tcp 7575
port -a -t gravity_agent_port_t -r 's0' -p tcp 3012
port -a -t gravity_port_t -r 's0' -p tcp 3022-3025
port -a -t gravity_port_t -r 's0' -p tcp 3008-3011
port -a -t gravity_port_t -r 's0' -p tcp 3080
port -a -t gravity_port_t -r 's0' -p tcp 32009
port -a -t gravity_kubernetes_port_t -r 's0' -p tcp 2379-2380
port -a -t gravity_kubernetes_port_t -r 's0' -p tcp 6443
port -a -t gravity_kubernetes_port_t -r 's0' -p tcp 7373
port -a -t gravity_kubernetes_port_t -r 's0' -p tcp 7496
port -a -t gravity_kubernetes_port_t -r 's0' -p tcp 10248-10255
port -a -t gravity_vxlan_port_t -r 's0' -p tcp 8472

fcontext -d -f a '{{.Path}}(/.*)?'
fcontext -d -f f '{{.Path}}/gravity'
fcontext -d -f f '{{.Path}}/gravity-(install|system)\.log'
fcontext -d -f a '{{.Path}}/.gravity(/.*)?'
fcontext -d -f f '{{.Path}}/.gravity/gravity-(installer|agent)\.service'
fcontext -d -f s '{{.Path}}/.gravity/installer\.sock'
fcontext -d -f f '{{.Path}}/crashreport(.*)?\.tgz'

fcontext -a -f a -t gravity_install_home_t -r 's0' '{{.Path}}(/.*)?'
fcontext -a -f f -t gravity_exec_t -r 's0' '{{.Path}}/gravity'
fcontext -a -f f -t gravity_log_t -r 's0' '{{.Path}}/gravity-(install|system)\.log'
fcontext -a -f a -t gravity_home_t -r 's0' '{{.Path}}/.gravity(/.*)?'
fcontext -a -f f -t gravity_unit_file_t -r 's0' '{{.Path}}/.gravity/gravity-(installer|agent)\.service'
fcontext -a -f s -t gravity_var_run_t -r 's0' '{{.Path}}/.gravity/installer\.sock'
fcontext -a -f f -t gravity_home_t -r 's0' '{{.Path}}/crashreport(.*)?\.tgz'
`))
