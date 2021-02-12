#!/bin/bash
#
# This file provides the following features to the tele build process:
#
#  1) Atomicity.  The TARGET is either complete or absent. (As long as
#     BUILD_TMP is on the same filesystem as TARGET and that fs supports atomic
#     renames). This means no half finished files for Make to trip on.
#  2) Isolation.  Various old teles *always* check ~/.tsh for a logged in cluster
#     and use it if available.  There is no way to disable this behavior.  Thus we
#     sandbox these processes in a container where ~/.tsh doesn't exist.
#
# The following environment variables must be specified by the caller:
#
# BUILD_TMP - Partially constructed images are built here. Must be on the same filesystem as TARGET for atomic move operations.
# TARGET - Where the cluster image should end up
# TELE  - The tele to build the image, should typically be equal to VERSION
# APP_YAML - The image manifest to be built.
# VERSION - The application version to use in the cluster image. Must be within SRCDIR.
# STATE_DIR - The gravity state dir where packages are cached/drawn from. May be shared across subsequent builds.
# EXTRA_TELE_BUILD_OPTIONS - Any additional flags to pass to tele build (e.g. for enterprise specific behavior).
set -o errexit
set -o nounset
set -o pipefail

EXTRA_TELE_BUILD_OPTIONS=${EXTRA_TELE_BUILD_OPTIONS:-}
TMP=$(mktemp -d -p "$BUILD_TMP")
trap "rm -rf $TMP" exit

TGT=$TMP/$(basename "$TARGET")

set -o xtrace
"$TELE" build \
$EXTRA_TELE_BUILD_OPTIONS \
"$APP_MANIFEST" \
--state-dir="$STATE_DIR" \
--version="$VERSION" \
--output="$TGT"

mv "$TGT" "$TARGET"
