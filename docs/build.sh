#!/bin/bash

# IMPORTANT! To add a new version, say 8.x
#     * copy 7.x.yaml to 8.x.yaml
#     * edit 8.x.yaml
#     * edit theme/scripts.html and update docVersions variable

cd $(dirname $0)
rm -f latest.yaml

# will be set to the latest version after the loop below
doc_ver=""

# find all *.yaml files and convert them to array, pick the latest
cfiles=$(ls *.yaml | sort)
cfiles_array=($cfiles)
latest_cfile=$(echo ${cfiles_array[-1]}) # becomes "7.x.yaml"
latest_ver=${latest_cfile%.yaml}         # becomes "7.x"

# build all documentation versions at the same time (4-8x speedup)
parallel --will-cite mkdocs build --config-file ::: $cfiles

# drop the 'latest.yml' symlink to the latest version so `mkdocs serve` will
# automatically serve the latest
echo "Latest version: $latest_ver"
ln -fs $latest_cfile latest.yaml

# copy the index file which serves /docs requests and redirects
# visitors to the latest verion of QuickStart
# cp index.html ../build/docs/index.html

# create a symlink called 'latest' to the latest directory, like "7.x"
cd ../build/docs
rm -f latest
ln -s $latest_ver latest

echo "The docs have been built and saved in 'build/docs'"