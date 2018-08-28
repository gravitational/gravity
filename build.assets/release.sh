#!/bin/bash
set -e

TEMP_DIR="$(mktemp -d)"
trap "rm -rf ${TEMP_DIR}" exit

cp ${TSH_OUT} ${TELE_OUT} install.sh ${TEMP_DIR}
tar -C ${TEMP_DIR} -zcvf ${RELEASE_OUT} .
