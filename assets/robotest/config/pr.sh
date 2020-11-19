#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/lib/utils.sh

# UPGRADE_MAP maps gravity version -> list of linux distros to upgrade from
declare -A UPGRADE_MAP
UPGRADE_MAP[$(recommended_upgrade_tag $(branch 7.0.x))]="centos:7" # compatible LTS version
UPGRADE_MAP[7.0.13]="centos:7" # 7.0.13 + centos is combination that is critical in the field -- 2020-07 walt
UPGRADE_MAP[7.0.12]="ubuntu:18" # 7.0.12 is the first LTS 7.0 release
UPGRADE_MAP[7.0.7]="ubuntu:16" # 7.0.7 is the first 7.0 with https://github.com/gravitational/planet/pull/671 included
# UPGRADE_MAP[7.0.0]="ubuntu:16" # 7.0.0 is prone to upgrade failure without https://github.com/gravitational/planet/pull/671

# 6.2 and 6.3 ignored in PR builds per https://github.com/gravitational/gravity/pull/1760#pullrequestreview-437838773
# UPGRADE_MAP[$(recommended_upgrade_tag $(branch 6.3.x))]="redhat:7" # compatible non-LTS version
# UPGRADE_MAP[6.3.0]="ubuntu:16"  # disabled due to https://github.com/gravitational/gravity/issues/1009
# UPGRADE_MAP[$(recommended_upgrade_tag $(branch 6.2.x))]="redhat:7" # compatible non-LTS version
# UPGRADE_MAP[6.2.0]="ubuntu:16"

function build_upgrade_suite {
  local size='"flavor":"three","nodes":3,"role":"node"'
  local to_tarball=${INSTALLER_URL}
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


function build_resize_suite {
  local suite=$(cat <<EOF
 resize={"installer_url":"${INSTALLER_URL}","nodes":1,"to":3,"flavor":"one","role":"node","state_dir":"/var/lib/telekube","os":"ubuntu:18","storage_driver":"overlay2"}
 shrink={"installer_url":"${INSTALLER_URL}","nodes":3,"flavor":"three","role":"node","os":"redhat:7"}
EOF
)
    echo -n $suite
}

function build_ops_suite {
  local suite=$(cat <<EOF
 install={"installer_url":"${OPSCENTER_URL}","nodes":1,"flavor":"standalone","role":"node","os":"ubuntu:18","ops_advertise_addr":"example.com:443"}
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
  local test_os="redhat:8.3"
  local cluster_size='"flavor":"three","nodes":3,"role":"node"'
  suite+=$(cat <<EOF
 install={"installer_url":"${INSTALLER_URL}",${cluster_size},"os":"${test_os}","storage_driver":"overlay2"}
EOF
)
  suite+=' '
  echo -n $suite
}

if [[ ${1} == "upgradeversions" ]] ; then
    UPGRADE_VERSIONS=${!UPGRADE_MAP[@]}
    echo "$UPGRADE_VERSIONS"
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
