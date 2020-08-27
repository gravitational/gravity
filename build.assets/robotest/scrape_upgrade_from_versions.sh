#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

source $1 > /dev/null
for release in ${!UPGRADE_MAP[@]}; do
  echo $release
done
