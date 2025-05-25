package network

import (
	"context"
	"strconv"
	"sync"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/config"
	"github.com/danmuck/dps_net/network/routing"
	"github.com/danmuck/dps_net/network/transport"
)

type RPC_Handler func(ctx context.Context, payload []byte) ([]byte, error)

type NetworkManager struct {
	nodeID    api.NodeID            // local nodeID
	localAddr string                // local network address <ip:port>
	router    *routing.RoutingTable // p2p network routing table
	info      *api.Contact

	udpRecv <-chan transport.Packet // UDP socket listener ** TYPE NOT IMPLEMENTED
	tcpRecv <-chan transport.Packet // TCP socket listener ** TYPE NOT IMPLEMENTED

	endpoints          map[string]*api.Contact                // address -> contact
	active             map[*api.Contact]bool                  // contact -> isActive
	appLocks           map[api.AppID]api.AppLock              // appID -> appLock
	appHandlerRegistry map[api.AppLock]map[string]RPC_Handler // name -> rpc name -> handler

	lock sync.Mutex
}

func NewNetworkManager(id api.NodeID) (*NetworkManager, error) {
	// much of this should probably me moved to node
	cfg, err := config.Load("")
	if err != nil {
		return nil, err
	}
	var tcpPort, udpPort string = "", ""
	var tcpRcv, udpRcv <-chan transport.Packet = nil, nil

	if cfg.TCPPort != 0 {
		tcpPort = strconv.Itoa(cfg.TCPPort)
		if tcpPort != "" {
			tcpsrv, err := transport.NewUDPServer(cfg.Address, tcpPort)
			if err != nil {
				return nil, err
			}
			tcpRcv = tcpsrv.Receiver()
		}

	}

	if cfg.UDPPort != 0 {
		udpPort = strconv.Itoa(cfg.UDPPort)
		if udpPort != "" {
			udpsrv, err := transport.NewUDPServer(cfg.Address, udpPort)
			if err != nil {
				return nil, err
			}
			udpRcv = udpsrv.Receiver()
		}

	}

	local := api.NewContact(id[:], cfg.Address, tcpPort, udpPort)
	nm := &NetworkManager{
		nodeID:    id,
		localAddr: cfg.Address,
		router:    routing.NewRoutingTable(local, cfg.K, cfg.Alpha),
		info:      local,

		udpRecv: udpRcv,
		tcpRecv: tcpRcv,

		endpoints:          make(map[string]*api.Contact),
		active:             make(map[*api.Contact]bool),
		appLocks:           make(map[api.AppID]api.AppLock),
		appHandlerRegistry: make(map[api.AppLock]map[string]RPC_Handler),
	}

	return nm, nil
}

func (m *NetworkManager) DispatchRPC(ctx context.Context, service, method string, payload []byte) ([]byte, error) {
	//
	return nil, nil
}
