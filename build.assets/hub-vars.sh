#!/bin/bash
set -u
cat > $GRAVITY_BUILDDIR/hub-vars.yaml <<EOF
binaries:
  gravity:
    path: $GRAVITY_OUT
    version: $GRAVITY_VERSION
  tele:
    path: $TELE_OUT
    version: $GRAVITY_VERSION
  tsh:
    path: $TSH_OUT
    version: $TELEPORT_TAG
apps:
  telekube:
    path: $TELEKUBE_OUT
    version: $GRAVITY_VERSION
release:
  path: $RELEASE_OUT
  version: $GRAVITY_VERSION
EOF
