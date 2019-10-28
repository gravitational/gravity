#!/bin/bash -e

function update_policy {
  time=`ls -l --time-style="+%x %X" $1.te | awk '{ printf "%s %s", $6, $7 }'`
  rules=`ausearch --start $time --message avc --raw --context $1`
  if [ x"$rules" != "x" ] ; then
    echo "Found AVCs to update policy with"
    echo -e "$rules" | audit2allow --reference
    echo "Do you want these changes added to policy [y/n]?"
    read ANS
    if [ "$ANS" = "y" -o "$ANS" = "Y" ] ; then
      echo "Updating policy for $1"
      echo -e "$rules" | audit2allow --reference >> $1.te
      # Fall though and rebuild policy
    else
      exit 0
    fi
  else
    echo "No new AVCs found"
    exit 0
  fi
}

function build_policy {
  echo "Building and loading Policy $1"
  set -x
  make -f /usr/share/selinux/devel/Makefile $1.pp || exit
  /usr/sbin/semodule --install $1.pp
}

function build_gravity_manpage {
  # Generate a man page off the installed module
  sepolicy manpage -p . -d gravity_t
}

function restore_gravity_fcontext {
  if [ -f /usr/bin/gravity ]; then
    # Fixing the file context on /usr/bin/gravity
    /sbin/restorecon -F -R -v /usr/bin/gravity
  fi

  # Fixing the file context on /var/lib/gravity
  /sbin/restorecon -F -R -v /var/lib/gravity
}

function build_gravity_rpm_package {
  # Generate a rpm package for the newly generated policy
  pwd=$(pwd)
  rpmbuild \
    --define "_sourcedir ${pwd}" \
    --define "_specdir ${pwd}" \
    --define "_builddir ${pwd}" \
    --define "_srcrpmdir ${pwd}" \
    --define "_rpmdir ${pwd}" \
    --define "_buildrootdir ${pwd}/.build" \
    -ba gravity_selinux.spec
}

DIRNAME=`dirname $0`
cd $DIRNAME
USAGE="$0 [ --update ]"
if [ `id --user` != 0 ]; then
  echo 'You must be root to run this script'
  exit 1
fi

if [ $# -eq 1 ]; then
  if [ "$1" = "--update" ] ; then
    update_policy gravity
    update_policy myconfineduser
  else
    echo -e $USAGE
    exit 1
  fi
elif [ $# -ge 2 ] ; then
  echo -e $USAGE
  exit 1
fi

build_policy gravity
build_gravity_manpage
restore_gravity_fcontext
build_gravity_rpm_package
