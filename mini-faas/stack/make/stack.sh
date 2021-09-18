#!/bin/bash
set -e
source ./stack/make/env.sh

if [ "$NODE_PROVIDER" == "local" ]; then
  source ./stack/make/start-resourcemanager-local.sh
else
  source ./stack/make/start-resourcemanager.sh
fi

source ./stack/make/start-scheduler.sh
source ./stack/make/start-apiserver.sh

start-resourcemanager
start-scheduler
start-apiserver

echo "Started the stack, apiserver endpoint $APISERVER_ENDPOINT"
