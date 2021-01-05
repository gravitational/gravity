#!/bin/bash
#
# This file is used to download tele binaries.  It provides the following benefits:
#
#  1) Atomicity.  The TARGET is either complete or absent. (So long as
#     BUILD_TMP is on the same filesystem as TARGET and that fs supports atomic
#     renames). This means no half finished files for Make to trip on.
#  2) The file can be overridden/replaced in the make file to allow enterprise
#     specific functionality.
#
# The following environment variables must be specified by the caller:
#
# BUILD_TMP - Used as staging while downloading images. Must be on the same filesystem as TARGET for atomic move operations.
# TARGET - Where the tele binary should end up
# VERSION - The tele version to download
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

TMP=$(mktemp -d -p "$BUILD_TMP")
trap "rm -rf \"$TMP\"" exit

wget --no-verbose "https://get.gravitational.io/telekube/bin/$VERSION/linux/x86_64/tele" --output-document "$TMP/tele"
chmod u+x "$TMP/tele"

mv "$TMP/tele" "$TARGET"
