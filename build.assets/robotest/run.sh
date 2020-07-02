#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

readonly TARGET=${1:?Usage: $0 [pr|nightly] [upgrade_from_dir]}
export UPGRADE_FROM_DIR=${2:-$(pwd)/../upgrade_from}

readonly GET_GRAVITATIONAL_IO_APIKEY=${GET_GRAVITATIONAL_IO_APIKEY:?API key for distribution Ops Center required}
readonly GRAVITY_BUILDDIR=${GRAVITY_BUILDDIR:?Set GRAVITY_BUILDDIR to the build directory}
readonly ROBOTEST_SCRIPT=$(mktemp -d)/runsuite.sh

# a number of environment variables are expected to be set
# see https://github.com/gravitational/robotest/blob/v2.0.0/suite/README.md
export ROBOTEST_VERSION=${ROBOTEST_VERSION:-2.0.0}
export ROBOTEST_REPO=quay.io/gravitational/robotest-suite:$ROBOTEST_VERSION
export INSTALLER_URL=$GRAVITY_BUILDDIR/telekube.tar
export GRAVITY_URL=$GRAVITY_BUILDDIR/gravity
export TAG=$(git rev-parse --short HEAD)
# cloud provider that test clusters will be provisioned on
# see https://github.com/gravitational/robotest/blob/v2.0.0/infra/gravity/config.go#L72
export DEPLOY_TO=${DEPLOY_TO:-gce}
export GCL_PROJECT_ID=${GCL_PROJECT_ID:-"kubeadm-167321"}
export GCE_REGION="northamerica-northeast1,us-west1,us-east1,us-east4,us-central1"
# GCE_VM tuned down from the Robotest's 7 cpu default in 09cec0e49e9d51c3603950209cec3c26dfe0e66b
# We should consider changing Robotest's default so that we can drop the override here. -- 2019-04 walt
export GCE_VM=${GCE_VM:-custom-4-8192}
# Parallelism & retry, tuned for GCE
export PARALLEL_TESTS=${PARALLEL_TESTS:-4}
export REPEAT_TESTS=${REPEAT_TESTS:-1}

# set SUITE and UPGRADE_VERSIONS
case $TARGET in
  pr) source $(dirname $0)/pr_config.sh;;
  nightly) source $(dirname $0)/nightly_config.sh;;
  *) echo "Unknown target $TARGET\nUsage: $0 [pr|nightly] [upgrade_from_dir]"; exit 1;;
esac

function build_volume_mounts {
  for release in ${UPGRADE_VERSIONS[@]}; do
      local tarball=$(tag_to_tarball ${release})
      echo "-v $UPGRADE_FROM_DIR/$tarball:/$tarball"
  done
}

export EXTRA_VOLUME_MOUNTS=$(build_volume_mounts)

tele=$GRAVITY_BUILDDIR/tele
mkdir -p $UPGRADE_FROM_DIR
for release in ${!UPGRADE_MAP[@]}; do
  $tele pull telekube:$release --output=$UPGRADE_FROM_DIR/telekube_$release.tar --hub=https://get.gravitational.io:443 --token="$GET_GRAVITATIONAL_IO_APIKEY"
done

docker pull $ROBOTEST_REPO
docker run $ROBOTEST_REPO cat /usr/bin/run_suite.sh > $ROBOTEST_SCRIPT
chmod +x $ROBOTEST_SCRIPT
$ROBOTEST_SCRIPT "$SUITE"
