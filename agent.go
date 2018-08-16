package main

import (
	"net"
	"os"
	"os/signal"

	"github.com/crosbymichael/boss/agent"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var agentCommand = cli.Command{
	Name:  "agent",
	Usage: "run the boss agent",
	Action: func(clix *cli.Context) error {
		s := make(chan os.Signal, 32)
		signal.Notify(s, os.Interrupt)

		c, err := config.Load()
		if err != nil {
			return err
		}
		client, err := system.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		store, err := c.Store()
		if err != nil {
			return err
		}
		a, err := agent.New(c, client, store)
		if err != nil {
			return err
		}
		server := newServer()
		v1.RegisterAgentServer(server, a)
		go func() {
			<-s
			server.Stop()
		}()
		l, err := net.Listen("tcp", clix.GlobalString("agent"))
		if err != nil {
			return err
		}
		defer l.Close()
		return server.Serve(l)
	},
}

func newServer() *grpc.Server {
	s := grpc.NewServer()

	hs := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, hs)
	return s
}
