#!/bin/sh
# Examine the output of git diff --raw to determine whether any files
# which match the pattern '^docs/' were changed.

REVLIST=$1
if [ -z "$REVLIST" ]; then
    echo "$0: please supply a git rev-list"
    echo "For more info see: git help rev-list"
    exit 2
fi

echo "---> git diff --raw ${REVLIST}"
git diff --raw ${REVLIST}
if [ $? -ne 0 ]; then
    echo "---> Unable to determine diff"
    exit 2
fi
git diff --raw ${REVLIST} | awk '{print $6}' | grep -E '^docs/' | wc -l > /tmp/.change_count.txt
export CHANGE_COUNT=$(cat /tmp/.change_count.txt | tr -d '\n')
rm /tmp/.change_count.txt
echo "---> Docs changes detected: $CHANGE_COUNT"
if [ "$CHANGE_COUNT" -gt 0 ]; then
    exit 1
else
    exit 0
fi
