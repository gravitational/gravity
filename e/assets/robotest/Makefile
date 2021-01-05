# This Makefile exists to make developing enterprise robotest
# targets easy. It exports the correct enterprise variables and offers
# granualar robotest targets (unlike the top level Makefiles, which only
# offer high level 'run' targets).
#
TOP := $(realpath $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))))
ROOT := $(realpath $(TOP)/../..)

# If any exported variables change, make sure they're updated in the root
# enterprise Gravity Makefile too.

# := needed to ensure the following variables are expanded under the current
# TOP/ROOT, because TOP/ROOT will be redefined in the included Makefile.
# ?= lazily and recursively expands variables.
ifeq ($(origin BUILDDIR), undefined)
BUILDDIR := $(ROOT)/build
endif
ifeq ($(origin ROBOTEST_DOWNLOAD_TELE_SCRIPT), undefined)
ROBOTEST_DOWNLOAD_TELE_SCRIPT := $(TOP)/download_tele.sh
endif
ROBOTEST_CACHE_FLAVOR ?= ent

export

include ../../../assets/robotest/Makefile
