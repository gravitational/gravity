#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/utils.sh

# UPGRADE_MAP maps gravity version -> list of linux distros to upgrade from
declare -A UPGRADE_MAP

UPGRADE_MAP[$(recommended_upgrade_tag $(branch 7.0.x))]="centos:7 redhat:7 debian:9 ubuntu:18" # compatible LTS version
UPGRADE_MAP[7.0.13]="centos:7" # 7.0.13 + centos is combination that is critical in the field -- 2020-07 walt
UPGRADE_MAP[7.0.12]="ubuntu:18"  # 7.0.12 is the first LTS 7.0 release
UPGRADE_MAP[7.0.0]="ubuntu:16"

# 6.2 and 6.3 won't be supported upgrades to 7.1, but they're not *intentionally* broken yet
UPGRADE_MAP[$(recommended_upgrade_tag $(branch 6.3.x))]="centos:7" # compatible non-LTS version
# UPGRADE_MAP[6.3.0]="ubuntu:16"  # disabled due to https://github.com/gravitational/gravity/issues/1009
UPGRADE_MAP[$(recommended_upgrade_tag $(branch 6.2.x))]="centos:7" # compatible non-LTS version
UPGRADE_MAP[6.2.0]="ubuntu:16"

UPGRADE_VERSIONS=${!UPGRADE_MAP[@]}

function build_upgrade_size_suite {
  local to_tarball=$(tag_to_image current)
  local os="centos:7"
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
  local to_tarball=$(tag_to_image current)
  local os="centos:7"
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
 resize={"to":3,"flavor":"one","nodes":1,"role":"node","state_dir":"/var/lib/telekube","os":"ubuntu:18","storage_driver":"overlay2"}
 resize={"to":6,"flavor":"three","nodes":3,"role":"node","state_dir":"/var/lib/telekube","os":"ubuntu:18","storage_driver":"overlay2"}
 shrink={"nodes":3,"flavor":"three","role":"node","os":"redhat:7"}
EOF
)
  echo -n $suite
}

function build_ops_install_suite {
  local suite=$(cat <<EOF
 install={"installer_url":"${ROBOTEST_IMAGE_DIR_MOUNTPOINT}/opscenter-current.tar","nodes":1,"flavor":"standalone","role":"node","os":"ubuntu:18","ops_advertise_addr":"example.com:443"}
EOF
)
  echo -n $suite
}

function build_install_suite {
  local suite=''
  local test_os="redhat:7 centos:7 debian:9 ubuntu:16 ubuntu:18"
  local cluster_sizes=( \
    '"flavor":"three","nodes":3,"role":"node"' \
    '"flavor":"six","nodes":6,"role":"node"')
  for os in $test_os; do
    for size in ${cluster_sizes[@]}; do
      suite+=$(cat <<EOF
 install={${size},"os":"${os}","storage_driver":"overlay2"}
EOF
)
      suite+=' '
    done
  done
  suite+=$(build_ops_install_suite)
  echo -n $suite
}

SUITE="$(build_install_suite)"
SUITE+=" $(build_resize_suite)"
SUITE+=" $(build_upgrade_suite)"

# robotest wants the suite space seperated, but it is easier to eyeball when newline seperated
echo "$SUITE" | tr ' ' '\n'

export SUITE UPGRADE_VERSIONS
