#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# This file exists to exercise the run target without long build & test times.

if [[ ${1} == "upgradeversions" ]]  ; then
    echo
elif [[ ${1} == "configuration" ]]  ; then
    echo 'noop={"fail":false,"sleep":3}'
fi
