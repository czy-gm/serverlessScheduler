#!/bin/bash
set -e
source ./stack/make/env.sh

start-apiserver() {
  if [ -z "$SCHEDULER_ENDPOINT" ]
  then
      echo "SCHEDULER_ENDPOINT environment variable is required but not provided"
      exit 1
  fi

  docker rm -f aliyuncnpc 2> /dev/null || true 1> /dev/null
  docker run -d -v ${LOCAL_LOG_DIR}/apiserver/log:/aliyuncnpc/apiserver/log \
      --name aliyuncnpc-apiserver \
      -e SCHEDULER_ENDPOINT=$SCHEDULER_ENDPOINT \
      -e SERVICE_PORT=$APISERVER_PORT \
      "$IMAGE_NAMESPACE/apiserver:$IMAGE_TAG"

  apiserver_ipaddr=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' aliyuncnpc-apiserver`
  export APISERVER_ENDPOINT=$apiserver_ipaddr:$APISERVER_PORT
  echo "APIServer started, endpoint: $APISERVER_ENDPOINT"
}