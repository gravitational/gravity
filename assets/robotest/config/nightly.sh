#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/lib/utils.sh

# UPGRADE_MAP maps gravity version -> list of linux distros to upgrade from
declare -A UPGRADE_MAP

# Use a fixed tag until we cut our first non-pre-release, as recommended_upgrade_tag skips pre-releases
# UPGRADE_MAP[$(recommended_upgrade_tag $(branch 9.0.x))]="redhat:8.4 redhat:7.8 centos:8.4 centos:7.8 ubuntu:16 ubuntu:18 ubuntu:20 debian:9 debian:10"
UPGRADE_MAP[9.0.0-beta.2]="redhat:8.4 redhat:7.9 centos:8.4 centos:7.9 ubuntu:16 ubuntu:18 ubuntu:20 debian:9 debian:10"
UPGRADE_MAP[8.0.0-beta.1]="redhat:8.4 centos:7.9 ubuntu:18 ubuntu:20"
UPGRADE_MAP[7.1.0-alpha.6]="ubuntu:20"

function build_upgrade_size_suite {
  local to_tarball=${INSTALLER_URL}
  local os="centos:7.9"
  local cluster_sizes=( \
    '"flavor":"three","nodes":3,"role":"node"' \
    '"flavor":"six","nodes":6,"role":"node"' \
    '"flavor":"one","nodes":1,"role":"node"')
  local suite=''
  # local from_tarball=$(tag_to_image $(recommended_upgrade_tag $(branch 7.0.x)))
  local from_tarball=$(tag_to_image 9.0.0-beta.2)
  for size in ${cluster_sizes[@]}; do
      suite+=$(build_upgrade_step $from_tarball $to_tarball $os $size)
    suite+=' '
  done
  echo -n $suite
}

function build_upgrade_to_release_under_test_suite {
  local to_tarball=${INSTALLER_URL}
  local size='"flavor":"three","nodes":3,"role":"node"'
  local suite=''
  for release in ${!UPGRADE_MAP[@]}; do
    local from_tarball=$(tag_to_image $release)
    for os in ${UPGRADE_MAP[$release]}; do
      suite+=$(build_upgrade_step $from_tarball $to_tarball $os $size)
      suite+=' '
    done
  done
  echo -n $suite
}

function build_upgrade_suite {
  local suite=$(cat <<EOF
 $(build_upgrade_size_suite)
 $(build_upgrade_to_release_under_test_suite)
EOF
)
  echo -n $suite
}

function build_resize_suite {
  local suite=$(cat <<EOF
 resize={"installer_url":"${INSTALLER_URL}","to":3,"flavor":"one","nodes":1,"role":"node","state_dir":"/var/lib/telekube","os":"ubuntu:18"}
 resize={"installer_url":"${INSTALLER_URL}","to":6,"flavor":"three","nodes":3,"role":"node","state_dir":"/var/lib/telekube","os":"redhat:7.9"}
 shrink={"installer_url":"${INSTALLER_URL}","nodes":3,"flavor":"three","role":"node","os":"redhat:7.9"}
EOF
)
  echo -n $suite
}

function build_ops_suite {
  local suite=$(cat <<EOF
 install={"installer_url":"${OPSCENTER_URL}","nodes":3,"flavor":"ha","role":"node","os":"ubuntu:18","ops_advertise_addr":"example.com:443"}
EOF
)
  echo -n $suite
}

function build_telekube_suite {
  local suite=$(cat <<EOF
 install={"installer_url":"${TELEKUBE_URL}","nodes":3,"flavor":"three","role":"node","os":"ubuntu:18"}
EOF
)
  echo -n $suite
}
function build_install_suite {
  local suite=''
  local oses="redhat:8.4 redhat:7.8 centos:8.4 centos:7.8 ubuntu:16 ubuntu:18 ubuntu:20 debian:9 debian:10"
  local cluster_sizes=( \
    '"flavor":"six","nodes":6,"role":"node"')
  for os in $oses; do
    for size in ${cluster_sizes[@]}; do
      suite+=$(cat <<EOF
 install={"installer_url":"${INSTALLER_URL}",${size},"os":"${os}"}
EOF
)
      suite+=' '
    done
  done
  echo -n $suite
}

if [[ ${1} == "upgradeversions" ]] ; then
    UPGRADE_VERSIONS=${!UPGRADE_MAP[@]}
    echo "$UPGRADE_VERSIONS"
    exit
elif [[ ${1} == "configuration" ]] ; then
    SUITE=""
    SUITE+=" $(build_telekube_suite)"
    SUITE+=" $(build_ops_suite)"
    SUITE+=" $(build_install_suite)"
    SUITE+=" $(build_resize_suite)"
    SUITE+=" $(build_upgrade_suite)"
    echo "$SUITE"
else
    echo "Unknown parameter: $1"
    exit 1
fi
