#!/bin/bash
set -euo pipefail
IMAGE_ID=$1
errorValue=0

if ! hash jq 2>/dev/null
then
    echo "'jq' was not found in PATH"
    echo "jq is needed to process the image id"
    errorValue=1
fi


if [[ "$#" -lt 1 ]]; then
   echo 
   echo "usage: getimageclassification.sh <imageclassifyids..222 52>"
   echo "set RES_CLASSIFICATION to the classification json file. Default ./class.json"
   errorValue=1
fi

RES_CLASSIFICATION=${RES_CLASSIFICATION:-./class.json}

if [ ! -f "$RES_CLASSIFICATION" ]; then
   echo "Classification file: $RES_CLASSIFICATION not available"
   errorValue=1
fi

if [ "$errorValue" -eq "1" ]; then
   exit 1 
fi


for var in "$@"
do
echo "Tensor result id retrieved: $var"
imageId=$(($var-1))
classificationName=$(cat $RES_CLASSIFICATION | jq -c ".[\"${imageId}\"] | .[1]" | sed 's,\",,g')
echo "Classification name: $classificationName"
 
done

