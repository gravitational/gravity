#!/bin/bash
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

if [ -z "$RES_CLASSIFICATION" ]; then
   RES_CLASSIFICATION="./class.json"
fi

if [ ! -f "$RES_CLASSIFICATION" ]; then
   echo "Classification file: $RES_CLASSIFICATION not available"
   errorValue=1
fi

if [ "$errorValue" -eq "1" ]; then
   exit 0 
fi


for var in "$@"
do
echo "Tensor result id retrieve: $var"
imageId=$(($var-1))
classificationName=$( echo "cat $RES_CLASSIFICATION | jq -c '.[\"$imageId\"] | .[1] '" | bash | sed "s,\",,g" )
echo "Classification name: $classificationName"
 
done

