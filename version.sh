#!/bin/bash

# this versioning algo:
#  - if on a tagged commit, use the tag
#  - if last tag was a regular relase, bump the minor version, make a it pre-release, and append # of commits since tag
#  - if last tag was a pre-release tag, append number of commits since the tag

increment_patch() {
    # increment_patch returns x.y.(z+1) given valid x.y.z semver.
    # If we need to robustly handle this, it is probably worth
    # looking at https://github.com/davidaurelio/shell-semver/
    # or moving this logic to a 'real' programming language -- 2020-03 walt
    major=$(echo $1 | cut -d'.' -f1)
    minor=$(echo $1 | cut -d'.' -f2)
    patch=$(echo $1 | cut -d'.' -f3)
    patch=$((patch + 1))
    echo "${major}.${minor}.${patch}"
}

SHORT_TAG=`git describe --abbrev=0 --tags`
LONG_TAG=`git describe --tags`
COMMIT_WITH_LAST_TAG=`git show-ref --tags --dereference | grep "refs/tags/${SHORT_TAG}^{}" | awk '{print $1}'`
COMMITS_SINCE_LAST_TAG=`git rev-list  ${COMMIT_WITH_LAST_TAG}..HEAD --count`

if [[ "$LONG_TAG" == "$SHORT_TAG" ]] ; then  # the current commit is tagged as a release
    echo "$SHORT_TAG"
elif [[ "$SHORT_TAG" != *-* ]] ; then  # the current ref is not a decendent of a pre-release version
    SHORT_TAG=$(increment_patch ${SHORT_TAG})
    echo "$SHORT_TAG-${COMMITS_SINCE_LAST_TAG}"
else  # the current ref is a decendent of a pre-release version (e.g. already an rc, alpha, or beta)
    echo "$SHORT_TAG.${COMMITS_SINCE_LAST_TAG}"
fi
