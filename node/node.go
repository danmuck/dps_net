package node

import (
	"context"
	"fmt"
	"strconv"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/config"
	"github.com/danmuck/dps_net/network"
)

// type Node struct {
// 	info *api.Contact
// 	net  network.NetworkManager

// 	storage api.StorageInterface
// 	cache   api.StorageInterface

// 	lock sync.RWMutex
// }

// func NewNode(id []byte, address string) *Node {
// 	n := &Node{
// 		info: api.NewContact(id, address, "", ""),
// 	}
// 	return n
// }

// Node is the primary entrypoint to the P2P network.
// It delegates routing and transport to the NetworkManager.
// All Kademlia state lives inside the manager's router service.
type Node struct {
	ID      api.NodeID
	Contact *api.Contact

	mgr *network.NetworkManager

	// application storage layers
	Storage api.StorageInterface
	Cache   api.StorageInterface

	ctx    context.Context
	cancel context.CancelFunc
}

// NewNode creates a Node, loading config from cfgPath (or auto-discovering when empty).
// It initializes the NetworkManager, which in turn sets up the routing table and Kademlia service.
func NewNode(cfgPath string) (*Node, error) {
	// Load config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Parse NodeID
	nid, err := api.StringToNodeID(cfg.NodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node_id: %w", err)
	}

	// Build local contact
	contact := api.NewContact(
		nid[:],
		cfg.Address,
		func() string {
			if cfg.TCPPort != 0 {
				return strconv.Itoa(cfg.TCPPort)
			}
			return ""
		}(),
		func() string {
			if cfg.UDPPort != 0 {
				return strconv.Itoa(cfg.UDPPort)
			}
			return ""
		}(),
	)

	// Initialize network manager with embedded routing and Kademlia service
	nm, err := network.NewNetworkManager(contact, *cfg)
	if err != nil {
		return nil, fmt.Errorf("init network manager: %w", err)
	}

	// Node context for cancellations
	nodeCtx, cancel := context.WithCancel(context.Background())

	n := &Node{
		ID:      nid,
		Contact: contact,
		mgr:     nm,
		// storage implementations can be injected or defaulted here
		// Storage: api.NewBoltStorage(),
		// Cache:   api.NewMemoryStorage(),
		ctx:    nodeCtx,
		cancel: cancel,
	}
	return n, nil
}

// Start launches transports and bootstraps via the manager.
func (n *Node) Start() error {
	// Start gRPC (if enabled) and UDP listeners
	// if err := n.mgr.StartGRPC(); err != nil {
	// 	return fmt.Errorf("start grpc: %w", err)
	// }
	// n.mgr.StartUDP()

	// Bootstrap with configured peers
	cfg, _ := config.Load("") // should reuse same cfg or store on Node
	for _, peer := range cfg.BootstrapPeers {
		if err := n.Join(peer); err != nil {
			fmt.Printf("warning: join %s: %v\n", peer, err)
		}
	}
	return nil
}

// Stop gracefully shuts down networking
func (n *Node) Stop() {
	n.cancel()
}

// Join sends a Ping RPC to integrate a peer into the routing table
func (n *Node) Join(peerAddr string) error {
	req := &api.RPC{
		Service: "services.KademliaService",
		Method:  "Ping",
		Sender:  n.Contact,
		Payload: nil,
	}
	var res api.RPC
	if err := n.mgr.InvokeRemote(n.ctx, peerAddr, req.Service, req.Method, req, &res); err != nil {
		return fmt.Errorf("ping %s: %w", peerAddr, err)
	}
	return nil
}

// Put stores key/value via the DHT
func (n *Node) Put(key, value []byte) error {
	// req := &api.RPC{
	// 	Service: "services.KademliaService",
	// 	Method:  "Store",
	// 	Sender:  n.Contact,
	// 	Payload: func() []byte {
	// 		// marshal STORE message… omitted
	// 		return nil
	// 	}(),
	// }

	// needs to find node and invoke the store on each node

	// var res api.RPC
	// if err := n.mgr.InvokeRemote(n.ctx /*peerAddr*/, "", req, &res); err != nil {
	// 	return fmt.Errorf("store rpc: %w", err)
	// }
	return nil
}

// Get fetches a value via the DHT
func (n *Node) Get(key []byte) ([]byte, bool, error) {
	req := &api.RPC{
		Service: "services.KademliaService",
		Method:  "FindValue",
		Sender:  n.Contact,
		Payload: func() []byte {
			// var r &
			// marshal FIND_VALUE message… omitted
			return nil
		}(),
	}
	var res api.RPC
	if err := n.mgr.InvokeRemote(n.ctx /*peerAddr*/, string(n.Contact.GetId()), req.Service, req.Method, req, &res); err != nil {
		return nil, false, fmt.Errorf("findvalue rpc: %w", err)
	}
	// unmarshal VALUE message from res.Payload… omitted
	return nil, false, nil
}
