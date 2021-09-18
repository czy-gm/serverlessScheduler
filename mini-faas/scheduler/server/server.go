package server

import (
	"aliyun/serverless/mini-faas/scheduler/core"
	pb "aliyun/serverless/mini-faas/scheduler/proto"
	"fmt"
	"golang.org/x/net/context"
	"sync"
)

type Server struct {
	sync.WaitGroup
	router *core.Router
}

func NewServer(router *core.Router) *Server {
	return &Server{
		router: router,
	}
}

func (s *Server) Start() {
	// Just in case the router has internal loops.
	s.router.Start()
}

func (s *Server) AcquireContainer(ctx context.Context, req *pb.AcquireContainerRequest) (*pb.AcquireContainerReply, error) {
	fmt.Printf("[INFO] acquire container. %v\n", req)
	reply, err := s.router.AcquireContainer(req)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[INFO] acquire container end! request id:%s, node id: %s, containerId:%s\n",
		req.GetRequestId(), reply.GetNodeId(), reply.GetContainerId())
	return reply, nil
}

func (s *Server) ReturnContainer(ctx context.Context, req *pb.ReturnContainerRequest) (*pb.ReturnContainerReply, error) {
	err := s.router.ReturnContainer(req)
	if err != nil {
		return nil, err
	}
	return &pb.ReturnContainerReply{}, nil
}
