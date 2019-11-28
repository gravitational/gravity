package selinux

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/log"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Bootstrap executes the bootstrap script for the specified directory
func Bootstrap(workingDir string) error {
	path, err := tempFilename(workingDir)
	if err != nil {
		return trace.Wrap(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_EXCL, defaults.SharedExecutableMask)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	if err := WriteBootstrapScript(f); err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command("bash", path)
	logger := log.New(logrus.WithField(trace.Component, "system:selinux"))
	w := logger.Writer()
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// WriteBootstrapScript creates the bootstrap script using the specified writer
func WriteBootstrapScript(w io.Writer) error {
	_, err := io.WriteString(w, bootstrapScript)
	return trace.ConvertSystemError(err)
}

func tempFilename(dir string) (filename string, err error) {
	f, err := ioutil.TempFile(dir, "bootstrap")
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	if err := f.Close(); err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return f.Name(), nil
}

const bootstrapScript = `#/bin/bash
set -eu

DIR="$( cd "$(dirname "$0")" ; pwd -P )"

function setup_file_contexts {
  # Label the current directory for installer
  semanage fcontext -a -t gravity_install_home_t "${DIR}(/.*)?"
  # Label the installer
  semanage fcontext -a -t gravity_exec_t -f f "${DIR}/gravity"
  semanage fcontext -a -t gravity_log_t -f f "${DIR}/gravity-(install|system)\.log"
  semanage fcontext -a -t gravity_home_t "${DIR}/.gravity(/.*)?"
  semanage fcontext -a -t gravity_unit_file_t -f f "${DIR}/.gravity/gravity-(installer|agent)\.service"
  semanage fcontext -a -t gravity_var_run_t -f s "${DIR}/.gravity/installer\.sock"
  semanage fcontext -a -t gravity_home_t -f f "${DIR}/crashreport(.*)?\.tgz"
  # Apply labels
  restorecon -Rv "${DIR}"
  restorecon -Rv /var/lib/gravity
}

function restore_file_contexts {
  semanage fcontext -d "${DIR}(/.*)?"
  semanage fcontext -d -f f "${DIR}/gravity"
  semanage fcontext -d -f f "${DIR}/gravity-(install|system)\.log"
  semanage fcontext -d "${DIR}/.gravity(/.*)?"
  semanage fcontext -d -f f "${DIR}/.gravity/gravity-(installer|agent)\.service"
  semanage fcontext -d -f s "${DIR}/.gravity/installer\.sock"
  semanage fcontext -d -f f "${DIR}/crashreport(.*)?\.tgz"
  restorecon -Rv "${DIR}"
}

function setup_ports {
  # https://danwalsh.livejournal.com/10607.html
  # Installer-specific ports
  semanage port -a -t gravity_install_port_t -p tcp 61009-61010
  semanage port -a -t gravity_install_port_t -p tcp 61022-61025
  semanage port -a -t gravity_install_port_t -p tcp 61080
  # Cluster ports
  # Gravity RPC agent
  semanage port -a -t gravity_agent_port_t -p tcp 3012
  semanage port -a -t gravity_agent_port_t -p tcp 7575
  # Gravity Hub control panel
  semanage port -a -t gravity_port_t -p tcp 32009
  # Gravity (teleport internal SSH control plane)
  semanage port -a -t gravity_port_t -p tcp 3022-3025
  # Gravity (teleport web UI)
  semanage port -a -t gravity_port_t -p tcp 3080
  # Gravity (internal gravity services)
  semanage port -a -t gravity_port_t -p tcp 3008-3011
  # Gravity (vxlan)
  semanage port -a -t gravity_vxlan_port_t -p tcp 8472
  # serf peer-to-peer
  semanage port -a -t gravity_kubernetes_port_t -p tcp 7373
  semanage port -a -t gravity_kubernetes_port_t -p tcp 7496
  semanage port -a -t gravity_kubernetes_port_t -p tcp 6443
  # reserved and overridden in the policy
  # semanage port -a -t gravity_docker_port_t -p tcp 5000
  # Kubernetes (etcd)
  semanage port -a -t gravity_kubernetes_port_t -p tcp 2379-2380
  # reserved and overridden in the policy
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 4001
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 7001
  # Kubernetes (apiserver)
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 6443
  # Kubernetes (kubelet)
  semanage port -a -t gravity_kubernetes_port_t -p tcp 10248-10255
}

function remove_ports {
  semanage port -d -t gravity_install_port_t -p tcp 61009-61010
  semanage port -d -p tcp 61022-61025
  semanage port -d -p tcp 61080
  semanage port -d -p tcp 3012
  semanage port -d -p tcp 7575
  semanage port -d -p tcp 32009
  semanage port -d -p tcp 3022-3025
  semanage port -d -p tcp 3080
  semanage port -d -p tcp 3008-3011
  semanage port -d -p tcp 8472
  semanage port -d -p tcp 7373
  semanage port -d -p tcp 7496
  semanage port -d -p tcp 6443
  semanage port -d -p tcp 2379-2380
  semanage port -d -p tcp 10248-10255
}

setup_file_contexts
setup_ports
`
