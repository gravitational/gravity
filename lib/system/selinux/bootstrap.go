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
		if err := os.Remove(f.Name()); err != nil {
			logrus.WithError(err).Warn("Failed to clean up bootstrap script.")
		}
	}()
	if err := WriteBootstrapScript(f); err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command("bash", path)
	logger := log.New(logrus.WithField(trace.Component, "system:selinux"))
	w := logger.Writer()
	cmd.Stdout = w
	cmd.Stderr = w
	return trace.Wrap(cmd.Run())
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
set -o nounset
set -o errexit
set -o xtrace

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

function upsert_port {
  local setype=$1
  local porttype=$2
  local range=$3
  output=$(semanage port -a -t $setype -p $porttype $range 2>&1 1>/dev/null)
  ret=$?
  if [[ $ret -eq 0 ]]; then
    return 0
  fi
  if ! echo $output | grep 'already defined'; then
    return $ret
  fi
  semanage port -m -t $setype -p $porttype $range
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
  set +o errexit
  # https://danwalsh.livejournal.com/10607.html
  # Installer-specific ports
  upsert_port gravity_install_port_t tcp 61009-61010
  upsert_port gravity_install_port_t tcp 61022-61025
  upsert_port gravity_install_port_t tcp 61080
  # Cluster ports
  # Gravity RPC agent
  upsert_port gravity_agent_port_t tcp 3012
  upsert_port gravity_agent_port_t tcp 7575
  # Gravity Hub control panel
  upsert_port gravity_port_t tcp 32009
  # Gravity (teleport internal SSH control plane)
  upsert_port gravity_port_t tcp 3022-3025
  # Gravity (teleport web UI)
  upsert_port gravity_port_t tcp 3080
  # Gravity (internal gravity services)
  upsert_port gravity_port_t tcp 3008-3011
  # Gravity (vxlan)
  upsert_port gravity_vxlan_port_t tcp 8472
  # serf peer-to-peer
  upsert_port gravity_kubernetes_port_t tcp 7373
  upsert_port gravity_kubernetes_port_t tcp 7496
  upsert_port gravity_kubernetes_port_t tcp 6443
  # reserved and overridden in the policy
  # semanage port -a -t gravity_docker_port_t -p tcp 5000
  # Kubernetes (etcd)
  upsert_port gravity_kubernetes_port_t tcp 2379-2380
  # reserved and overridden in the policy
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 4001
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 7001
  # Kubernetes (apiserver)
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 6443
  # Kubernetes (kubelet)
  upsert_port gravity_kubernetes_port_t tcp 10248-10255
  set -o errexit
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
