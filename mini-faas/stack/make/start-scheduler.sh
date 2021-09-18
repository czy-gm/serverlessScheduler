#!/bin/bash
set -e
source ./stack/make/env.sh

if [ -z "$SCHEDULER_IMAGE" ]
then
  SCHEDULER_IMAGE=registry.cn-hangzhou.aliyuncs.com/aliyun-cnpc-faas-dev/scheduler-demo:v1beta1
fi

if [ -z "$SCHEDULER_PORT" ]
then
    export SCHEDULER_PORT=10600
fi

start-scheduler() {
  if [ -z "$RESOURCE_MANAGER_ENDPOINT" ]
  then
      echo "RESOURCE_MANAGER_ENDPOINT environment variable is required but not provided"
      exit 1
  fi

  resourcemanager_endpoint=$1
  docker rm -f aliyuncnpc-scheduler 2> /dev/null || true 1> /dev/null
  docker run -d -v ${LOCAL_LOG_DIR}/scheduler/log:/aliyuncnpc/scheduler/log \
    --name aliyuncnpc-scheduler \
    -e SERVICE_PORT=$SCHEDULER_PORT \
    -e RESOURCE_MANAGER_ENDPOINT=$RESOURCE_MANAGER_ENDPOINT \
    -e STACK_NAME=$STACK_NAME \
    "$SCHEDULER_IMAGE"

  scheduler_ipaddr=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' aliyuncnpc-scheduler`
  endpoint=$scheduler_ipaddr:$SCHEDULER_PORT
  echo "Scheduler started, endpoint: $endpoint"
  export SCHEDULER_ENDPOINT=$endpoint
}

