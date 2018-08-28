#!/bin/bash

# this versioning algo:
# keeps tag as is in case if this version is an equal match
# otherwise adds .<number of commits since last tag>

SHORT_TAG=`git describe --abbrev=0 --tags`
LONG_TAG=`git describe --tags`
COMMIT_WITH_LAST_TAG=`git show-ref --tags --dereference | grep "refs/tags/${SHORT_TAG}^{}" | awk '{print $1}'`
COMMITS_SINCE_LAST_TAG=`git rev-list  ${COMMIT_WITH_LAST_TAG}..HEAD --count`

if [[ "$LONG_TAG" == "$SHORT_TAG" ]] ; then
    echo "$SHORT_TAG"
elif [[ "$SHORT_TAG" != *-* ]] ; then
    echo "$SHORT_TAG-${COMMITS_SINCE_LAST_TAG}"
else
    echo "$SHORT_TAG.${COMMITS_SINCE_LAST_TAG}"
fi
