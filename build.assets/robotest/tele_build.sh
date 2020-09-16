#!/bin/bash
#
# This file provides the following features to the tele build process:
#
#  1) Atomicity.  The TARGET is either complete or absent. (So long as long as
#     BUILD_TMP is on the same filesystem as TARGET and that fs supports atomic
#     renames). This means no half finished files for Make to trip on.
#
# The following environment variables must be specified by the caller:
#
# BUILD_TMP - Partially constructed images are built here. Must be on the same filesystem as TARGET for atomic move operations.
# TARGET - Where the cluster image should end up
# TELE  - The tele to build the image, should typically be equal to VERSION
# VERSION - The application version to use in the cluster image
# APP_YAML - The image manifest to be built.
# STATE_DIR - The gravity state dir where packages are cached/drawn from. May be a shared with parallel invocations.
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

TMP=$(mktemp -d -p $BUILD_TMP)
trap "rm -rf $TMP" exit
TGT=$TMP/$(basename $TARGET)

$TELE build \
    $APP_YAML \
    --state-dir=$STATE_DIR \
    --version=$VERSION \
    --output=$TGT
mv $TGT $TARGET
