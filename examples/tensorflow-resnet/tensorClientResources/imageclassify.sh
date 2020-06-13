#!/bin/bash
EXAMPLE_NAME=$1
IMAGE_ID=$2
errorValue=0

if ! hash dos2unix 2>/dev/null
then
    echo "'dos2unix' was not found in PATH"
    echo "dos2unix is needed to process the image id"
    errorValue=1
fi
if ! hash jq 2>/dev/null
then
    echo "'jq' was not found in PATH"
    echo "jq is needed to process the image id"
    errorValue=1
fi


if [ "$#" -lt 2 ]; then
   echo 
   echo "usage: <request id> <url to image> <optional server:port>"
   echo "if server is not given the env variable RESNET_SERVER is used"
   errorValue=1
fi


if [ "$errorValue" -eq "1" ]; then
   exit 0 
fi

if [ "$#" -lt 3 ]; then
   SERVER="$RESNET_SERVER"
else
   SERVER=$3
fi


cp tensorflow_serving/example/exampletemplate.py tensorflow_serving/example/${EXAMPLE_NAME}.py 
sed -i "s,IMAGE_ID_REPLACE,$IMAGE_ID,g" tensorflow_serving/example/${EXAMPLE_NAME}.py
echo "Requesting classification for image: $IMAGE_ID"
echo "Using server: $SERVER"
imageClassification=$(tools/run_in_docker.sh -d tensorflow/serving:1.14.0-devel python tensorflow_serving/example/${EXAMPLE_NAME}.py --server=${SERVER}  | grep int64_val: | awk '{print $2 }')
# the image id comes back with a dos carriage return that causes problem in the echo
imageId=$(echo $imageClassification | dos2unix)
# the image is actually 1 less in the classes file
imageId=$((imageId-1))

if [ "$imageId" -eq "-1" ]; then
   echo "Classification not possible.  Confirm image is available at that url."
   exit 0
fi

echo "Classification: $imageId"
classificationName=$( echo "cat class.json | jq -c '.[\"$imageId\"] | .[1] '" | bash | sed "s,\",,g" )
echo "Classification name: $classificationName"






