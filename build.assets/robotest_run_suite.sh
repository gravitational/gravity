#!/bin/bash
set -eu -o pipefail

readonly UPGRADE_FROM_DIR=${1:-$(pwd)/../upgrade_from}

declare -A UPGRADE_FROM
# gravity version -> list of OS releases to exercise on
UPGRADE_FROM[5.0.24]="centos:7.5 ubuntu:latest"

readonly GET_GRAVITATIONAL_IO_APIKEY=${GET_GRAVITATIONAL_IO_APIKEY:?API key for distribution Ops Center required}
readonly GRAVITY_BUILDDIR=${GRAVITY_BUILDDIR:?Set GRAVITY_BUILDDIR to the build directory}
readonly ROBOTEST_SCRIPT=$(mktemp -d)/runsuite.sh

# number of environment variables are expected to be set
# see https://github.com/gravitational/robotest/blob/master/suite/README.md
export ROBOTEST_VERSION=${ROBOTEST_VERSION:-stable}
export ROBOTEST_REPO=quay.io/gravitational/robotest-suite:$ROBOTEST_VERSION
export WAIT_FOR_INSTALLER=true
export INSTALLER_URL=$GRAVITY_BUILDDIR/telekube.tar
export DEPLOY_TO=${DEPLOY_TO:-azure}
export TAG=$(git rev-parse --short HEAD)
export GCL_PROJECT_ID=${GCL_PROJECT_ID:-"kubeadm-167321"}
export AZURE_REGION=${AZURE_REGION:-"westus,westus2,centralus,canadacentral"}

function build_resize_suite {
  cat <<EOF
 resize={"to":3,"flavor":"one","nodes":1,"role":"node","state_dir":"/var/lib/telekube","os":"ubuntu:latest","storage_driver":"devicemapper"}
EOF
}

function build_upgrade_step {
  local usage="$FUNCNAME os release"
  local os=${1:?$usage}
  local release=${2:?$usage}
  local cluster_sizes=('"flavor":"three","nodes":3,"role":"node"' '"flavor":"six","nodes":6,"role":"node"' '"flavor":"one","nodes":1,"role":"node"')
  local suite=''
  for size in ${cluster_sizes[@]}; do
    suite+=$(cat <<EOF
 upgrade3lts={${size},"os":"${os}","storage_driver":"overlay2","from":"/telekube_${release}.tar"}
EOF
)
  done
  echo $suite
}

function build_devicemapper_upgrade_step {
  local usage="$FUNCNAME os release"
  local os=${1:?$usage}
  local release=${2:?$usage}
  local suite=''
  suite+=$(cat <<EOF
 upgrade3lts={"flavor":"three","nodes":3,"role":"node","os":"${os}","storage_driver":"devicemapper","from":"/telekube_${release}.tar"}
EOF
)
  echo $suite
}

function build_upgrade_suite {
  local suite=''
  for release in ${!UPGRADE_FROM[@]}; do
    for os in ${UPGRADE_FROM[$release]}; do
      suite+=$(build_upgrade_step $os $release)
      suite+=' '
    done
    suite+=$(build_devicemapper_upgrade_step 'redhat:7.4' $release)
    suite+=' '
  done
  echo $suite
}

function build_install_suite {
  local suite=''
  local test_os="redhat:7.4 centos:7.5 ubuntu:latest"
  local cluster_sizes=('"flavor":"three","nodes":3,"role":"node"' '"flavor":"six","nodes":6,"role":"node"')
  local storage_drivers="overlay2 devicemapper"
  for os in $test_os; do
    for driver in $storage_drivers; do
      for size in ${cluster_sizes[@]}; do
        suite+=$(cat <<EOF
 install={${size},"os":"${os}","storage_driver":"${driver}"}
EOF
)
      done
    done
  done
  suite+=$(cat <<EOF
 install={"installer_url":"/installer/opscenter.tar","nodes":1,"flavor":"standalone","role":"node","os":"ubuntu:latest","ops_advertise_addr":"example.com:443"}
EOF
)
  echo $suite
}

function build_volume_mounts {
  for release in ${!UPGRADE_FROM[@]}; do
    echo "-v $UPGRADE_FROM_DIR/telekube_${release}.tar:/telekube_${release}.tar"
  done
}

export EXTRA_VOLUME_MOUNTS=$(build_volume_mounts)

suite=$(build_resize_suite)
suite="$suite $(build_upgrade_suite)"
suite="$suite $(build_install_suite)"

echo $suite

mkdir -p $UPGRADE_FROM_DIR
tele login --ops=https://get.gravitational.io:443 --key="$GET_GRAVITATIONAL_IO_APIKEY"
for release in ${!UPGRADE_FROM[@]}; do
  tele pull telekube:$release --output=$UPGRADE_FROM_DIR/telekube_$release.tar
done

docker pull $ROBOTEST_REPO
docker run $ROBOTEST_REPO cat /usr/bin/run_suite.sh > $ROBOTEST_SCRIPT
chmod +x $ROBOTEST_SCRIPT
$ROBOTEST_SCRIPT "$suite"
