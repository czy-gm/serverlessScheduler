// Package client implements the details of rpc communication with Scheduler
// Agent Server.
package client

import (
	"google.golang.org/grpc"

	pb "aliyun/serverless/mini-faas/scheduler/proto"
)

// Client closable SchedulerClient interface
type Client interface {
	pb.SchedulerClient
	Close()
}

// client is a rpc proxy to talk to scheduler service.
type client struct {
	pb.SchedulerClient

	// TODO: use connection pool?
	conn *grpc.ClientConn
}

// Factory factory to build a Client type
type Factory func(serverAddr string) (Client, error)

// New creates a Client object.
func New(serverAddr string) (Client, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return &client{
		conn:            conn,
		SchedulerClient: pb.NewSchedulerClient(conn),
	}, nil
}

// Close releases any resources the client holds.
func (c *client) Close() {
	c.conn.Close()
}
