#!/bin/bash

# This file contains unit tests for the upgrade recommendation algorithm.
#
# None of the values in this file need to be kept up to date as new releases
# come out, because the algorithm should never recommend a release that is
# chronologically more recent than the release being upgraded to.  Because all
# test cases use a specific tag (e.g. 7.0.10) for "to" instead of a floating
# ref (e.g. 7.0.x) the recommended "from" releases should be fixed, regardless of
# new releases on the same branch.
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

source $(dirname $0)/utils.sh

# Check recommended upgrade where the from branch has more current etcd
# versions than the to release.
#
# 7.0.0 supports upgrade from etcd 3.3.12 as its most current 3.3.
# 6.1.25 bumps etcd to 3.3.20, meaning 6.1.24 is the last 6.1.x that can upgrade
# to 7.0.0.
#
# See https://github.com/gravitational/gravity/issues/1735 for more info.
#
# Due to difficulty of determining etcd compatibility, this test uses 6.1.20 as a
# conservative stand-in for 6.1.24 because 6.1.20 the last 6.1.x that came out before 7.0.0.
if [[ "$(recommended_upgrade_tag_between $(branch 6.1.x) 7.0.0)" != "6.1.20" ]];  then
  exit 1
fi

# Check a more current 7.0.x upgrades from a more current 6.1.x.
#
# 7.0.11 supports upgrade from etcd 3.3.22, which is what 6.1.29 uses.
if [[ "$(recommended_upgrade_tag_between $(branch 6.1.x) 7.0.11)" != "6.1.29" ]];  then
  exit 1
fi

# Check that an upgrade from a release to itself is not recommended.
if [[ "$(recommended_upgrade_tag_between $(branch 7.0.x) 7.0.10)" != "7.0.9" ]];  then
  exit 1
fi

# Check recommended upgrade when patch-1  doesn't exist.
#
# 6.1.23 is the only recent case of this in the wild.
if [[ "$(recommended_upgrade_tag_between $(branch 6.1.x) "6.1.24")" != "6.1.22" ]];  then
  exit 1
fi
