package net

import (
	"sync"

	"github.com/danmuck/dps_net/api"
)

type NetworkManager struct {
	nodeID    api.NodeID        // local nodeID
	address   string            // local network address <ip:port>
	router    *api.RoutingTable // p2p network routing table
	neighbors []*api.Contact    // active/recent connections

	udpListener string // UDP socket listener ** TYPE NOT IMPLEMENTED
	tcpListener string // TCP socket listener ** TYPE NOT IMPLEMENTED

	lock sync.Mutex
}
