#!/bin/bash

# IMPORTANT! To add a new version, say 6.x
#     * copy 5.x directory to 6.x and make changes there
#     * copy 5.x.yaml to 6.x.yaml and make changes there
#     * edit theme/base.html and update the docVersions variable

cd "$(dirname $0)";

# enumerate all available versions and create a symlink to the latest
# i.e. latest.yaml -> version.yaml
rm -f latest.yaml
versions=$(ls *.yaml | sort)
for latest in $versions; do
    echo "Found version: $latest"
done
echo "Latest version --> $latest"
ln -fs $latest latest.yaml

#
# executed with 'run' argument?
# run the latest version interactively:
#
if [ "$1" == "run" ] ; then
    trap "exit" INT TERM ERR
    trap "kill 0" EXIT

    echo "Starting mkdocs server..."
    mkdocs serve --config-file $latest --livereload --dev-addr=0.0.0.0:6600 &
    wait
    exit
fi

#
# build all versions
#
for version in $versions; do
    doc_ver=${version%.yaml}
    echo "Building docs version --> $doc_ver"
    mkdocs build --config-file $version || exit $?
done
echo "SUCCESS --> build/docs"


# copy the index file which serves /docs requests and redirects
# visitors to the latest verion of QuickStart
cp index.html ../build/docs/index.html

# create a symlink to the latest
cd ../build/docs
rm -f latest
ln -s $doc_ver latest
