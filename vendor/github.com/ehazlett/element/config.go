package element

import (
	"io/ioutil"
	"log"
	"net"
	"strconv"

	"github.com/hashicorp/memberlist"
)

// ConnectionType defines the type of connection for the agent to use (wan, lan, local)
type ConnectionType string

const (
	// LAN is a local area network connection intended for high speed, low latency networks
	LAN ConnectionType = "lan"
	// WAN is a wide area connection intended for long distance remote links
	WAN ConnectionType = "wan"
	// Local is a local connection intended for high speed local development links
	Local ConnectionType = "local"
)

// Config is the agent config
type Config struct {
	// NodeName is the name of the node.  Each node must have a unique name in the cluster.
	NodeName string
	// Address on which the agent will serve the GRPC services
	Address string
	// ConnectionType is the connection type the agent will use
	ConnectionType string
	// ClusterAddress bind address
	ClusterAddress string
	// AdvertiseAddress for nat traversal
	AdvertiseAddress string
	// Peers is a local cache of peer members
	Peers []string
	// Debug output for memberlist
	Debug bool
}

func (a *Agent) Config() *Config {
	return a.config
}

func (cfg *Config) memberListConfig(peerUpdateChan chan bool, nodeEventChan chan *NodeEvent) (*memberlist.Config, error) {
	var mc *memberlist.Config
	switch cfg.ConnectionType {
	case string(Local):
		mc = memberlist.DefaultLocalConfig()
	case string(WAN):
		mc = memberlist.DefaultWANConfig()
	case string(LAN):
		mc = memberlist.DefaultLANConfig()
	default:
		return nil, ErrUnknownConnectionType
	}

	mc.Name = cfg.NodeName
	mc.Delegate = NewAgentDelegate(cfg.NodeName, cfg.Address, peerUpdateChan, nodeEventChan)
	mc.Events = NewEventHandler(nodeEventChan)

	if !cfg.Debug {
		mc.Logger = log.New(ioutil.Discard, "", 0)
	}

	host, port, err := net.SplitHostPort(cfg.ClusterAddress)
	if err != nil {
		return nil, err
	}
	// ml overrides for connection
	if v := host; v != "" {
		mc.BindAddr = host
	}
	if v := port; v != "" {
		mc.BindPort, _ = strconv.Atoi(port)
	}
	if host, port, err = net.SplitHostPort(cfg.ClusterAddress); err != nil {
		return nil, err
	}
	if v := host; v != "" {
		mc.AdvertiseAddr = host
	}
	if v := port; v != "" {
		mc.AdvertisePort, _ = strconv.Atoi(port)
	}
	return mc, nil
}
