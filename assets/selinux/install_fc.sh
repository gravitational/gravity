#/bin/bash

# if [ `id --user` != 0 ]; then
#   echo 'You must be root to run this script'
#   exit 1
# fi

DIR="$( cd "$(dirname "$0")" ; pwd -P )"

function setup_file_contexts {
  # Label the current directory for installer
  semanage fcontext -a -t gravity_installer_t "${DIR}(/.*)?"
  # Label the installer
  semanage fcontext -a -t gravity_exec_t "${DIR}/gravity"
  semanage fcontext -a -t gravity_log_t "${DIR}/gravity-(install|system)\.log"
  semanage fcontext -a -t gravity_state_t "${DIR}/.gravity"
  semanage fcontext -a -t gravity_unit_t "${DIR}/.gravity/gravity-(installer|agent)\.service"
  semanage fcontext -a -t gravity_state_t "${DIR}/crashreport(.*)?\.tgz"
  # Apply labels
  restorecon -Rv "${DIR}"
}

function setup_ports {
  # https://danwalsh.livejournal.com/10607.html
  # Installer-specific ports
  semanage port -m -t gravity_installer_port_t -p tcp 61009-61010
  semanage port -m -t gravity_installer_port_t -p tcp 61022-61025
  semanage port -m -t gravity_installer_port_t -p tcp 61080
  # Cluster ports
  # Gravity RPC agent
  semanage port -m -t gravity_agent_port_t -p tcp 3012
  semanage port -m -t gravity_agent_port_t -p tcp 7575
  # Gravity Hub control panel
  semanage port -m -t gravity_port_t -p tcp 32009
  # Gravity (teleport internal SSH control plane)
  semanage port -m -t gravity_port_t -p tcp 3022-3025
  # Gravity (teleport web UI)
  semanage port -m -t gravity_port_t -p tcp 3080
  # Gravity (internal gravity services)
  semanage port -m -t gravity_port_t -p tcp 3008-3011
  # Gravity (vxlan)
  semanage port -m -t gravity_vxlan_port_t -p tcp 8472
  # serf peer-to-peer
  semanage port -m -t gravity_kubernetes_port_t -p tcp 7373
  semanage port -m -t gravity_kubernetes_port_t -p tcp 7496
  # reserved and overridden in the policy
  # semanage port -a -t gravity_docker_port_t -p tcp 5000
  # Kubernetes (etcd)
  semanage port -m -t gravity_kubernetes_port_t -p tcp 2379-2380
  # reserved and overridden in the policy
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 4001
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 7001
  # Kubernetes (apiserver)
  # semanage port -a -t gravity_kubernetes_port_t -p tcp 6443
  # Kubernetes (kubelet)
  semanage port -m -t gravity_kubernetes_port_t -p tcp 10248-10255
}

setup_file_contexts
setup_ports

# TODO
