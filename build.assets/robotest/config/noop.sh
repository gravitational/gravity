#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# This file exists to exercise the run target without long build & test times.

export UPGRADE_VERSIONS=""

# The following breaks a dependency loop. We need upgrade versions to generate tarballs,
# but we need tarball names to generate the full test config.
if [[ ${1} == "upgradeexit" ]]  ; then
    return
fi

SUITE='noop={"fail":false,"sleep":3}'

echo "$SUITE" | tr ' ' '\n'

export SUITE
