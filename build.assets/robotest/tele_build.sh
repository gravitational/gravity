#!/bin/bash
#
# This file provides the following features to the tele build process:
#
#  1) Atomicity.  The TARGET is either complete or absent. (So long as long as
#     BUILD_TMP is on the same filesystem as TARGET and that fs supports atomic
#     renames). This means no half finished files for Make to trip on.
#  2) Isolation.  Various old teles *always* check ~/.tsh for a logged in cluster
#     and use it if available.  There is no way to disable this behavior.  Thus we
#     sandbox these processes in a container where ~/.tsh doesn't exist.
#
# The following environment variables must be specified by the caller:
#
# IMAGE - A docker cotainer that the build will run in
# BUILD_TMP - Partially constructed images are built here. Must be on the same filesystem as TARGET for atomic move operations.
# TARGET - Where the cluster image should end up
# TELE  - The tele to build the image, should typically be equal to VERSION
# APP_SRCDIR - The directory that contains all files necessary to build the application.
# APP_YAML - The image manifest to be built.
# VERSION - The application version to use in the cluster image. Must be within SRCDIR.
# STATE_DIR - The gravity state dir where packages are cached/drawn from. May be a shared with parallel invocations.
# EXTRA_TELE_BUILD_OPTIONS - Any additional flags to pass to tele build (e.g. for enterprise specific behavior).
set -o errexit
set -o nounset
set -o pipefail

TMP=$(mktemp -d -p "$BUILD_TMP")
trap "rm -rf $TMP" exit

TGT=$TMP/$(basename "$TARGET")


NOROOT="--user=$(id -u):$(id -g) --group-add=$(getent group docker | cut -d: -f3)"
VOLUMES="-v $TELE:$TELE -v $APP_SRCDIR:$APP_SRCDIR -v $STATE_DIR:$STATE_DIR -v $TMP:$TMP"
VOLUMES="$VOLUMES -v /var/run/docker.sock:/var/run/docker.sock -v /run/docker.sock:/run/docker.sock"
VOLUMES="$VOLUMES -v $HOME/.docker:/.docker:ro"
VOLUMES="$VOLUMES --tmpfs /tmp"

(
set -o xtrace
docker run --rm=true --net=host $NOROOT \
    $VOLUMES \
    -w "$TMP" \
    $IMAGE \
    dumb-init \
    "$TELE" build \
    "$APP_MANIFEST" \
    --state-dir="$STATE_DIR" \
    --version="$VERSION" \
    --output="$TGT"

mv "$TGT" "$TARGET"
)
