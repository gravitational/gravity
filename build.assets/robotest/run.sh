#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

readonly TARGET=${1:?Usage: [path to config]}

export GRAVITY_URL=${GRAVITY_URL:?Set GRAVITY_URL to the current gravity binary}
export INSTALLER_URL=${INSTALLER_URL:?Set INSTALLER_URL to the default robotest image}
export ROBOTEST_IMAGES_DIR=${ROBOTEST_IMAGES_DIR:? Set ROBOTEST_IMAGES_DIR to the directory with robotest images}
readonly ROBOTEST_SCRIPT=$(mktemp -d)/runsuite.sh

# a number of environment variables are expected to be set
# see https://github.com/gravitational/robotest/blob/v2.0.0/suite/README.md
export ROBOTEST_VERSION=${ROBOTEST_VERSION:-2.1.0}
export ROBOTEST_REPO=quay.io/gravitational/robotest-suite:$ROBOTEST_VERSION
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

# set SUITE and ROBOTEST_IMAGE_DIR_MOUNTPOINT
source $TARGET

# ROBOTEST_IMAGE_DIR_MOUNTPOINT defined by the config
export EXTRA_VOLUME_MOUNTS="-v $ROBOTEST_IMAGES_DIR:$ROBOTEST_IMAGE_DIR_MOUNTPOINT"

docker pull $ROBOTEST_REPO
docker run $ROBOTEST_REPO cat /usr/bin/run_suite.sh > $ROBOTEST_SCRIPT
chmod +x $ROBOTEST_SCRIPT
$ROBOTEST_SCRIPT "$SUITE"
