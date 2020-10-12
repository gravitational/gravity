#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

readonly TARGET=${1:?Usage: [path to config]}

GRAVITY_URL=${GRAVITY_URL:?Set GRAVITY_URL to the current gravity binary}
IMAGEDIR_MOUNTPOINT=${IMAGEDIR_MOUNTPOINT:? Set IMAGEDIR_MOUNTPOINT to the mountpoint where images will be availabe inside the container}
INSTALLER_URL=${INSTALLER_URL:?Set INSTALLER_URL to the container local default robotest image}
OPSCENTER_URL=${OPSCENTER_URL:?Set OPSCENTER_URL to the container local opscenter image}
TELEKUBE_URL=${TELEKUBE_URL:?Set TELEKUBE_URL to the container local telekube image}
IMAGEDIR=${IMAGEDIR:? Set IMAGEDIR to the directory with robotest images}
STATEDIR=${STATEDIR:?Set STATEDIR to a suitable place to store robotest terraform state and logs}

TAG=$(git rev-parse --short HEAD)

# cloud provider that test clusters will be provisioned on
# see https://github.com/gravitational/robotest/blob/v2.0.0/infra/gravity/config.go#L72
DEPLOY_TO=${DEPLOY_TO:-gce}
GCL_PROJECT_ID=${GCL_PROJECT_ID:-"kubeadm-167321"}
GCE_REGION="northamerica-northeast1,us-west1,us-east1,us-east4,us-central1"
# GCE_VM tuned down from the Robotest's 7 cpu default in 09cec0e49e9d51c3603950209cec3c26dfe0e66b
# We should consider changing Robotest's default so that we can drop the override here. -- 2019-04 walt
GCE_VM=${GCE_VM:-custom-4-8192}
GCE_PREEMPTIBLE=${GCE_PREEMPTIBLE:-'false'}
# Parallelism & retry, tuned for GCE
PARALLEL_TESTS=${PARALLEL_TESTS:-4}
REPEAT_TESTS=${REPEAT_TESTS:-1}
RETRIES=${RETRIES:-3}
FAIL_FAST=${FAIL_FAST:-false}
ALWAYS_COLLECT_LOGS=${ALWAYS_COLLECT_LOGS:-true}

# what should happen with provisioned VMs on individual test success or failure
DESTROY_ON_SUCCESS=${DESTROY_ON_SUCCESS:-true}
DESTROY_ON_FAILURE=${DESTROY_ON_FAILURE:-true}

EXTRA_VOLUME_MOUNTS="-v $IMAGEDIR:$IMAGEDIR_MOUNTPOINT:ro"

export INSTALLER_URL TELEKUBE_URL OPSCENTER_URL IMAGEDIR_MOUNTPOINT
SUITE=$($TARGET configuration)

# GRAVITY_FILE/GRAVITY_URL specify the location of the up-to-date gravity binary
if [ -d $(dirname ${GRAVITY_URL}) ]; then
  GRAVITY_FILE='/robotest-bin/'$(basename ${GRAVITY_URL})
  EXTRA_VOLUME_MOUNTS=${EXTRA_VOLUME_MOUNTS:-}" -v "$(dirname ${GRAVITY_URL}):$(dirname ${GRAVITY_FILE}:ro)
fi

check_files () {
	ABORT=
	for v in $@ ; do
		if [ ! -f "${v}" ] ; then
			echo "${v} does not exist"
			ABORT=true
		fi
	done

	if [ ! -z $ABORT ] ; then
		exit 1 ;
	fi
}

if [ $DEPLOY_TO == "gce" ] ; then
check_files ${SSH_KEY} ${SSH_PUB} ${GOOGLE_APPLICATION_CREDENTIALS}

CUSTOM_VAR_FILE=$(mktemp)
trap "{ rm -f $CUSTOM_VAR_FILE; }" EXIT
cat <<EOF > $CUSTOM_VAR_FILE
{"preemptible": "${GCE_PREEMPTIBLE}"}
EOF
EXTRA_VOLUME_MOUNTS=${EXTRA_VOLUME_MOUNTS:-}" -v "$CUSTOM_VAR_FILE:/robotest/config/vars.json

GCE_CONFIG="gce:
  credentials: /robotest/config/creds.json
  vm_type: ${GCE_VM}
  region: ${GCE_REGION}
  ssh_key_path: /robotest/config/ops.pem
  ssh_pub_key_path: /robotest/config/ops_rsa.pub
  var_file_path: /robotest/config/vars.json"
fi

if [ -n "${GCL_PROJECT_ID:-}" ] ; then
	check_files ${GOOGLE_APPLICATION_CREDENTIALS}
fi

CLOUD_CONFIG="
installer_url: ${INSTALLER_FILE:-${INSTALLER_URL}}
gravity_url: ${GRAVITY_FILE:-${GRAVITY_URL}}
script_path: /robotest/terraform/${DEPLOY_TO}
state_dir: /robotest/state
cloud: ${DEPLOY_TO}
${AWS_CONFIG:-}
${GCE_CONFIG:-}
"

mkdir -p $STATEDIR

NOROOT="--user=$(id -u):$(id -g)"

set -o xtrace

exec docker run \
    $NOROOT \
	-v ${STATEDIR}:/robotest/state \
	-v ${SSH_KEY}:/robotest/config/ops.pem:ro \
	${GCE_CONFIG:+'-v' "${SSH_PUB}:/robotest/config/ops_rsa.pub:ro"} \
	${GCE_CONFIG:+'-v' "${GOOGLE_APPLICATION_CREDENTIALS}:/robotest/config/creds.json:ro"} \
	${GCL_PROJECT_ID:+'-v' "${GOOGLE_APPLICATION_CREDENTIALS}:/robotest/config/gcp.json:ro" '-e' 'GOOGLE_APPLICATION_CREDENTIALS=/robotest/config/gcp.json'} \
	${EXTRA_VOLUME_MOUNTS:-} \
	${DOCKER_IMAGE} \
	dumb-init robotest-suite -test.timeout=48h \
	${GCL_PROJECT_ID:+"-gcl-project-id=${GCL_PROJECT_ID}"} \
	-test.parallel=${PARALLEL_TESTS} -repeat=${REPEAT_TESTS} -retries=${RETRIES} -fail-fast=${FAIL_FAST} \
	-provision="${CLOUD_CONFIG}" -always-collect-logs=${ALWAYS_COLLECT_LOGS} \
	-resourcegroup-file=/robotest/state/alloc.txt \
	-destroy-on-success=${DESTROY_ON_SUCCESS} -destroy-on-failure=${DESTROY_ON_FAILURE} \
	-tag=${TAG} -suite=sanity \
	${SUITE}
