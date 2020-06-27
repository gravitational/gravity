#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

function branch {
  # GRAVITY_GIT_VERSION_BRANCH_PREFIX may defined to accommodate remote names other than"origin"
  local branch_prefix=${GRAVITY_GIT_VERSION_BRANCH_PREFIX:-remotes/origin/version}
  echo $branch_prefix/$1
}

function commit_hash_of {
  git rev-list -1 $1
}

function date_of {
  git show -s --format=%ci $1
}

function latest_release {
  # Pre-release tags are intentionally ignored.
  # We don't presently care about validating upgrades to/from them. -- walt 2020-06
  readonly REGULAR_RELEASE_REGEX='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
  local ref=${1:?$FUNCNAME ref}
  tag=$(git describe --abbrev=0 $ref)
  while [[ ! "$tag" =~ $REGULAR_RELEASE_REGEX ]]; do
    tag=$(git describe --abbrev=0 $tag^)
  done
  echo $tag
}

# recommended_upgrade_tag_between returns a recommended tag in from_ref which
# should be a safe Gravity upgrade to to_ref.
#
# Algorithm for determining safe Gravity upgrades:
#
# 1) end user provides line of development being upgraded from and
#    specific build being upgraded to as a git ref (branch, tag, hash)
# 2) find the date of the most recent commit X in the history of from_ref
#    that chronologically predates the tip of to_ref
# 3) find the most recent non-pre-release tag in the history of X
#
# This avoids several known failure modes:
# * #1735 etcd more recent in from than to. Avoided by chronological ordering.
# * Pre-release builds with unpublished artifacts. Many X.X.0-alpha.X
#   are git tagged, but not released. Avoided by ignoring pre-releases.
# * a release is skipped entirely. For example 6.1.23. Avoided by inspecting
#   tag history instead of a naive patch - 1.
#
# However there is still one known failure mode: A regular release that is
# tagged but lacks published robotest artifacts. At the time of writing there were
# several of these: 5.5.11, 5.5.16, 5.5.25, 5.5.29, 5.5.39, 5.6.2, 5.6.3
#
# After discussion with r0mant and knisbet, these look erroneous, and we want
# to fail CI if similar situations occur in the future.  We don't need to worry
# about these ones in the past, 5.5.x and 5.6.x both have more current releases
# that would be picked over any of these. -- 2020-06 walt
function recommended_upgrade_tag_between {
  local usage="$FUNCNAME from_ref to_ref"
  local from_ref=${1:?$usage}
  local to_ref=${2:?$usage}
  local before=$(date_of $(commit_hash_of $to_ref))
  local parallel_commit=$(git rev-list -1 --before="$before" $(commit_hash_of $from_ref))
  if [[ "$parallel_commit" == "$(commit_hash_of $to_ref)" ]]; then
    # to_ref is equal to or an ancestor of from_ref
    # Go up one tag, as upgrading from a release to itself isn't supported.
    parallel_commit="$parallel_commit^"
  fi
  local recommended=$(latest_release $parallel_commit)
  echo $recommended
}

# recommended_upgrade_tag inspects its argument and returns the best tag
# to test Gravity upgrade from the argument to the current commit.
function recommended_upgrade_tag {
  recommended_upgrade_tag_between $1 HEAD
}

function build_upgrade_step {
  local usage="$FUNCNAME from_tarball os cluster-size"
  local from_tarball=${1:?$usage}
  local os=${2:?$usage}
  local cluster_size=${3:?$usage}
  local storage_driver='"storage_driver":"overlay2"'
  local service_opts='"service_uid":997,"service_gid":994' # see issue #1279
  local suite=''
  suite+=$(cat <<EOF
 upgrade={${cluster_size},"os":"${os}","from":"$from_tarball",${service_opts},${storage_driver}}
EOF
)
  echo $suite
}

function semver_to_tarball {
  local version=${1:?specify a version}
  echo "telekube_${version}.tar"
}

