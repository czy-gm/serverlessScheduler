#!/bin/bash
set -e
source ./stack/make/env.sh

image=$IMAGE_NAMESPACE/sample-invoke:$IMAGE_TAG

if [ "$BUILD" == "true" ]; then
  docker run -v $(pwd):/mini-faas golang:1.12.9 bash -c "cd /mini-faas/sample/invoke; export GOPROXY=https://goproxy.io; go build -o sample-invoke ./main.go"
  cd sample/invoke
  chmod +x sample-invoke
  docker build -t $image .
  rm ./sample-invoke
fi

apiserver_ipaddr=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' aliyuncnpc-apiserver`
apiserver_endpoint=$apiserver_ipaddr:$APISERVER_PORT
echo apiserverEndpoint $apiserver_endpoint

docker run --name aliyuncnpc-sample-invoke $image -apiserverEndpoint $apiserver_endpoint