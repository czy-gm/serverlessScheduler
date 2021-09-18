#!/bin/bash

source ./stack/make/env.sh

echo "Cleaning the stack"
docker rm -f aliyuncnpc-apiserver; docker rm -f aliyuncnpc-scheduler; docker rm -f aliyuncnpc-resourcemanager; docker rm -f aliyuncnpc-sample-invoke
docker ps -a|grep aliyun-cnpc-nodeservice|awk '{print $1}'|xargs docker rm -f
docker ps -a|grep $IMAGE_NAMESPACE/containerserviceext:$IMAGE_TAG|awk '{print $1}'|xargs docker rm -f