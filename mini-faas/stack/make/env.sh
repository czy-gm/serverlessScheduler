if [ -z "$IMAGE_TAG" ]; then
    IMAGE_TAG=local-v1beta1
fi

if [ -z "$LOCAL_LOG_DIR" ]
then
    export LOCAL_LOG_DIR=$HOME/aliyuncnpc
fi

if [ -z "$IMAGE_NAMESPACE" ]
then
    export IMAGE_NAMESPACE=registry.cn-hangzhou.aliyuncs.com/aliyun-cnpc-faas-dev
fi

if [ -z "$NODE_SERVICE_ROOT_DIR" ]
then
    export NODE_SERVICE_ROOT_DIR=$HOME
    echo NODE_SERVICE_ROOT_DIR $NODE_SERVICE_ROOT_DIR
fi

if [ -z "$APISERVER_PORT" ]
then
    APISERVER_PORT=10500
fi
