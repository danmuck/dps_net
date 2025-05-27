package node

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/config"
	"github.com/danmuck/dps_net/network"
	"github.com/danmuck/dps_net/network/routing"
	// "github.com/danmuck/dps_net/network/routing"
)

// Node is the primary entrypoint to the P2P network.
// It delegates routing and transport to the NetworkManager.
// All Kademlia state lives inside the manager's router service.
type Node struct {
	ID      api.NodeID
	Contact *api.Contact
	cfg     *config.Config

	mgr *network.NetworkManager

	// application storage layers
	Storage api.StorageInterface
	Cache   api.StorageInterface

	ctx    context.Context
	cancel context.CancelFunc

	lock sync.RWMutex
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
	contact.UpdateUsername(cfg.Username)

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
		cfg:     cfg,
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
	n.mgr.Start()
	// Bootstrap with configured peers
	for _, peer := range n.cfg.BootstrapPeers {
		if err := n.Ping(peer); err != nil {
			fmt.Printf("warning: join %s: %v\n", peer, err)
		}
	}
	return nil
}

// Stop gracefully shuts down networking
func (n *Node) Stop() {
	n.mgr.Shutdown()
	n.cancel()
}

// Ping sends a Ping RPC to integrate a peer into the routing table
func (n *Node) Ping(peerAddr string) error {
	// 1) Build the typed request
	ping := &routing.PING{
		From:  n.Contact,
		Value: nil, // or []byte whatever payload you want
	}

	// 2) Prepare the typed response holder
	var ack routing.ACK

	// 3) InvokeRemote will marshal pingReq, wrap in api.RPC, send, then
	//    unmarshal the inner ACK for you
	if err := n.mgr.InvokeRPC(
		n.ctx,
		peerAddr,
		"routing.KademliaService", // service name
		"Ping",                    // method name
		ping,                      // typed request
		&ack,                      // typed response
	); err != nil {
		return fmt.Errorf("ping %s: %w", peerAddr, err)
	}

	return nil
}

// Join bootstraps this node into the network via the given peer.
// It:
//  1. Pings the bootstrap node
//  2. Asks it for the k closest nodes to you
//  3. Pings each of those to fill your buckets
func (n *Node) Join(bootstrapAddr string) error {
	// 0) ensure the bootstrap isn’t yourself
	if bootstrapAddr == n.Contact.GetUDPAddress() {
		return fmt.Errorf("cannot bootstrap to self")
	}

	// 1) Ping the bootstrap
	if err := n.Ping(bootstrapAddr); err != nil {
		return fmt.Errorf("cannot reach bootstrap %s: %w", bootstrapAddr, err)
	}

	// 2) Find the k closest nodes to *your* own ID
	peers, err := n.mgr.Lookup(n.ctx, n.ID)
	if err != nil {
		return fmt.Errorf("bootstrap Lookup via %s failed: %w", bootstrapAddr, err)
	}
	if len(peers) == 0 {
		// It’s not necessarily an error—if k=1 and only bootstrap exists, you’re done.
		log.Printf("Join: no additional peers returned after bootstrap")
	}

	// 3) Ping each returned peer in parallel (fill your buckets)
	var wg sync.WaitGroup
	for _, peer := range peers {
		addr := peer.GetAddress() + ":" + peer.GetUdpPort()
		// avoid pinging the bootstrap twice
		if addr == bootstrapAddr || addr == n.Contact.GetUDPAddress() {
			continue
		}
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			if err := n.Ping(addr); err != nil {
				log.Printf("warning: ping %s failed: %v", addr, err)
			}
		}(addr)
	}
	wg.Wait()
	return nil
}

// Put stores key/value via the DHT
func (n *Node) Put(key, value []byte) error {
	// req := &api.RPC{
	// 	Service: "routing.KademliaService",
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
		Service: "routing.KademliaService",
		Method:  "FindValue",
		Sender:  n.Contact,
		Payload: func() []byte {
			// var r &
			// marshal FIND_VALUE message… omitted
			return nil
		}(),
	}
	var res api.RPC
	if err := n.mgr.InvokeRPC(n.ctx /*peerAddr*/, string(n.Contact.GetId()), req.Service, req.Method, req, &res); err != nil {
		return nil, false, fmt.Errorf("findvalue rpc: %w", err)
	}
	// unmarshal VALUE message from res.Payload… omitted
	return nil, false, nil
}
