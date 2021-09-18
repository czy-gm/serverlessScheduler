package core

import (
	"fmt"
	cmap "github.com/orcaman/concurrent-map"
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	pb "aliyun/serverless/mini-faas/nodeservice/proto"
)

type NodeInfo struct {
	sync.Mutex
	nodeID              string
	address             string
	port                int64
	availableMem        int64
	usageMem            int64
	containerMap        cmap.ConcurrentMap // container_id -> containerInfo
	functionMap         cmap.ConcurrentMap // functionMap (functionName -> num)
	conn *grpc.ClientConn
	pb.NodeServiceClient
}

func NewNode(nodeID, address string, port, memory int64) (*NodeInfo, error) {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", address, port), grpc.WithInsecure())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &NodeInfo{
		Mutex:               sync.Mutex{},
		nodeID:              nodeID,
		address:             address,
		port:                port,
		availableMem:        memory,
		usageMem:            0,
		functionMap:         cmap.New(),
		containerMap:        cmap.New(),
		conn:                conn,
		NodeServiceClient:   pb.NewNodeServiceClient(conn),
	}, nil
}

// Close closes the connection
func (n *NodeInfo) Close() {
	n.conn.Close()
}
