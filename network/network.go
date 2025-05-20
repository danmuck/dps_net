package network

import (
	"context"
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

func (nm *NetworkManager) Ping(ctx context.Context, to api.Contact) error {
	return nil
}

func (nm *NetworkManager) FindNodeRPC(ctx context.Context, to api.Contact, target api.NodeID) ([]api.Contact, error) {
	return nil, nil
}

func (nm *NetworkManager) FindValueRPC(ctx context.Context, to api.Contact, key api.NodeID) (value []byte, closest []api.Contact, err error) {
	return nil, nil, nil
}

func (nm *NetworkManager) StoreRPC(ctx context.Context, to api.Contact, key api.NodeID, value []byte) error {
	return nil
}
