package main

import (
	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

type LocalAgent struct {
	v1.AgentClient
	conn *grpc.ClientConn
}

func (a *LocalAgent) Close() error {
	return a.conn.Close()
}

func Agent(clix *cli.Context) (*LocalAgent, error) {
	conn, err := grpc.Dial(clix.GlobalString("agent"), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &LocalAgent{
		AgentClient: v1.NewAgentClient(conn),
		conn:        conn,
	}, nil
}
