#!/bin/bash
set -e

TEMP_DIR="$(mktemp -d)"
trap "rm -rf ${TEMP_DIR}" exit

# These commands add the following assets to the release tarball:
# install.sh
# LICENSE
# README.md
# tele
# gravity
# tsh
# VERSION
cp ${GRAVITY_OUT} ${TSH_OUT} ${TELE_OUT} install.sh README.md ../LICENSE ${TEMP_DIR}
../version.sh > ${TEMP_DIR}/VERSION

tar -C ${TEMP_DIR} -zcvf ${RELEASE_OUT} .
