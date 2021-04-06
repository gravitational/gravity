#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/lib/utils.sh

# UPGRADE_MAP maps gravity version -> list of linux distros to upgrade from
declare -A UPGRADE_MAP
UPGRADE_MAP[7.1.0-alpha.5]="redhat:8.2 redhat:7.8 centos:8.2 centos:7.8 sles:12-sp5 sles:15-sp2 ubuntu:16 ubuntu:18 ubuntu:20 debian:9 debian:10"
UPGRADE_MAP[$(recommended_upgrade_tag $(branch 7.0.x))]="redhat:8.2 redhat:7.8 centos:8.2 centos:7.8 sles:12-sp5 sles:15-sp2 ubuntu:16 ubuntu:18 ubuntu:20 debian:9 debian:10"
UPGRADE_MAP[7.0.13]="centos:7" # 7.0.13 + centos is combination that is critical in the field -- 2020-07 walt
UPGRADE_MAP[7.0.12]="ubuntu:18"  # 7.0.12 is the first LTS 7.0 release
UPGRADE_MAP[7.0.0]="ubuntu:16"

# 6.3 won't be a supported upgrades to 7.1, it is not *intentionally* broken yet
UPGRADE_MAP[$(recommended_upgrade_tag $(branch 6.3.x))]="centos:7.9" # compatible non-LTS version
# UPGRADE_MAP[6.3.0]="ubuntu:16"  # disabled due to https://github.com/gravitational/gravity/issues/1009

function build_upgrade_size_suite {
  local to_tarball=${INSTALLER_URL}
  local os="centos:7.9"
  local cluster_sizes=( \
    '"flavor":"three","nodes":3,"role":"node"' \
    '"flavor":"six","nodes":6,"role":"node"' \
    '"flavor":"one","nodes":1,"role":"node"')
  local suite=''
  local from_tarball=$(tag_to_image $(recommended_upgrade_tag $(branch 7.0.x)))
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
  local oses="redhat:8.2 redhat:7.8 centos:8.2 centos:7.8 sles:12-sp5 sles:15-sp2 ubuntu:16 ubuntu:18 ubuntu:20 debian:9 debian:10"
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
