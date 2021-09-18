#!/bin/bash
set -e
source ./stack/make/env.sh

start-resourcemanager() {
  if [ -z "$RESOURCE_MANAGER_PORT" ]
  then
      RESOURCE_MANAGER_PORT=10400
  fi

  docker pull "$IMAGE_NAMESPACE/nodeservice:$IMAGE_TAG"
  docker pull "$IMAGE_NAMESPACE/containerserviceext:$IMAGE_TAG"
  docker run -d --name aliyuncnpc-resourcemanager \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v ${LOCAL_LOG_DIR}/resourcemanager/log:/aliyuncnpc/resourcemanager/log \
    -e NODE_SERVICE_ROOT_DIR=$NODE_SERVICE_ROOT_DIR \
    "$IMAGE_NAMESPACE/resourcemanager:$IMAGE_TAG"

  resourcemanager_ipaddr=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' aliyuncnpc-resourcemanager`
  endpoint="$resourcemanager_ipaddr:$RESOURCE_MANAGER_PORT"
  echo "ResourceManager started, endpoint: $endpoint"

  export RESOURCE_MANAGER_ENDPOINT=$endpoint
}
