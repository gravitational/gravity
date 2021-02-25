#!/bin/bash
# Copyright 2021 Gravitational Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
