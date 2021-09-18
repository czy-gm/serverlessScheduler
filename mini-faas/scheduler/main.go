package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"

	"google.golang.org/grpc"

	rmPb "aliyun/serverless/mini-faas/resourcemanager/proto"
	"aliyun/serverless/mini-faas/scheduler/core"
	pb "aliyun/serverless/mini-faas/scheduler/proto"
	"aliyun/serverless/mini-faas/scheduler/server"
	"aliyun/serverless/mini-faas/scheduler/utils/env"
)

const (
	defaultPort          = 10600
)

func main() {
	fmt.Println("[INFO] start scheduler server!")
	done := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())

	go env.HandleSignal(cancel, done)
	svr := grpc.NewServer()

	rmEndpoint := os.Getenv("RESOURCE_MANAGER_ENDPOINT")
	if rmEndpoint == "" {
		rmEndpoint = "0.0.0.0:10400"
	}
	fmt.Printf("[INFO] Creating resource manager client with endpoint %s\n", rmEndpoint)

	conn, err := grpc.Dial(rmEndpoint, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("[ERROR] Failed to contact resource manager due to %s\n", err)
		panic(err)
	}
	rm := rmPb.NewResourceManagerClient(conn)

	router := core.NewRouter(rm)
	s := server.NewServer(router)
	s.Start()

	pb.RegisterSchedulerServer(svr, s)

	servicePort := defaultPort
	servicePortS := os.Getenv("SERVICE_PORT")
	if servicePortS != "" {
		port, err := strconv.Atoi(servicePortS)
		if err != nil {
			panic(err)
		}
		servicePort = port
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", servicePort))
	if err != nil {
		fmt.Printf("[ERROR] Failed to listen on port %d: %v\n", servicePort, err)
		return
	}

	go svr.Serve(lis)

	select {
	case <-ctx.Done():
		fmt.Printf("[ERROR] Scheduler gRPC server gracefully stopping ...")
		svr.GracefulStop()
		fmt.Printf("[ERROR] Scheduler gRPC server gracefully stopped.")
	}
}
