package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/crosbymichael/boss/agent"
	"github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/config"
	"github.com/crosbymichael/boss/system"
	"github.com/crosbymichael/boss/util"
	"github.com/ehazlett/element"
	raven "github.com/getsentry/raven-go"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var agentCommand = cli.Command{
	Name:  "agent",
	Usage: "run the boss agent",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "agent-port,p",
			Usage: "agent port, the agent binds to all and you cannot change it",
			Value: 1337,
		},
		cli.IntFlag{
			Name:  "cluster-port",
			Usage: "the cluster port for gossip",
			Value: 1388,
		},
		cli.StringSliceFlag{
			Name:  "peers",
			Usage: "set the agent peers",
			Value: &cli.StringSlice{},
		},
		cli.StringFlag{
			Name:  "id",
			Usage: "set the agent id",
		},
	},
	Action: func(clix *cli.Context) error {
		s := make(chan os.Signal, 32)
		signal.Notify(s, os.Interrupt)

		c, err := config.Load()
		if err != nil {
			return err
		}
		id := clix.String("id")
		if id == "" {
			id = c.ID
		}
		ip, err := util.GetIP(c.Iface)
		if err != nil {
			return err
		}
		var (
			labels         = make(map[string]string)
			address        = fmt.Sprintf("%s:%d", ip, clix.Int("port"))
			clusterAddress = fmt.Sprintf("%s:%d", ip, clix.Int("cluster-port"))
			peers          = append(c.Agent.Peers, clix.StringSlice("peers")...)
		)
		if c.Agent.Master {
			labels[agent.Master] = ""
		}
		logrus.WithField("address", address).Debug("agent address")
		node, err := element.NewAgent(&element.Config{
			NodeName:         id,
			Address:          address,
			ConnectionType:   string(element.LAN),
			ClusterAddress:   clusterAddress,
			AdvertiseAddress: clusterAddress,
			Peers:            peers,
			Debug:            true,
			Labels:           labels,
		})
		if err != nil {
			return err
		}
		logrus.WithField("advertise", clusterAddress).Debug("agent cluster address")
		if err := node.Start(nil); err != nil {
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
		a, err := agent.New(c, client, store, node)
		if err != nil {
			return err
		}
		server := newServer()
		v1.RegisterAgentServer(server, a)
		go func() {
			<-s
			go node.Shutdown()
			server.Stop()
		}()
		l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", clix.Int("port")))
		if err != nil {
			return err
		}
		defer l.Close()
		return server.Serve(l)
	},
}

func newServer() *grpc.Server {
	s := grpc.NewServer(
		grpc.UnaryInterceptor(unary),
		grpc.StreamInterceptor(stream),
	)

	hs := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, hs)
	return s
}

func unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	r, err := grpc_prometheus.UnaryServerInterceptor(ctx, req, info, handler)
	if err != nil {
		raven.CaptureError(err, nil)
	}
	return r, err
}

func stream(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := grpc_prometheus.StreamServerInterceptor(srv, ss, info, handler)
	if err != nil {
		raven.CaptureError(err, nil)
	}
	return err
}
