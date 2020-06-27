#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

source $(dirname $0)/utils.sh

# see https://github.com/gravitational/gravity/issues/1735 for explanation of why
# 6.1.2x+ to 7.0.0 isn't supported.
if [[ "$(recommended_upgrade_tag_between $(branch 6.1.x) 7.0.0)" != "6.1.20" ]];  then
  exit 1
fi

# check a more current 7.0.x has a more current corresponding 6.0.x
if [[ "$(recommended_upgrade_tag_between $(branch 6.1.x) 7.0.11)" != "6.1.29" ]];  then
  exit 1
fi

# ensure an upgrade from a release to itself is not recommended
if [[ "$(recommended_upgrade_tag_between $(branch 7.0.x) 7.0.11)" != "7.0.10" ]];  then
  exit 1
fi
