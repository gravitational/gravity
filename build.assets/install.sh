#!/bin/sh
set -e

#
# The directory where Gravity binaries will be located.
#
BINDIR=/usr/local/bin

sudo cp -f tele tsh gravity $BINDIR
