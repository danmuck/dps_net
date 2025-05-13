package dht

import (
	"crypto/rand"
	"sync"
	"time"
)

type NodeID [20]byte

type Node struct {
	ID       NodeID
	Addr     string
	Routing  *RoutingTable
	Info     Contact
	Store    map[string][]byte
	Mutex    sync.RWMutex
	LastSeen time.Time
	Conn     *ConnManager
}

func NewNode(address string) *Node {
	id := GenerateRandomID()
	node := &Node{
		ID:       id,
		Addr:     address,
		Routing:  NewRoutingTable(id),
		Info:     Contact{ID: id, Address: address},
		Store:    make(map[string][]byte),
		LastSeen: time.Now(),
	}
	node.Conn = NewConnManager(node)
	return node
}

func GenerateRandomID() NodeID {
	var id NodeID
	rand.Read(id[:])
	return id
}

func (n *Node) Ping(remoteAddr string) bool {
	// Stub for sending PING message

	return true
}

func (n *Node) StoreValue(key string, value []byte) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	n.Store[key] = value
}

func (n *Node) FindNode(target NodeID) []string {
	// Stub: Return closest nodes to target ID
	return []string{}
}

func (n *Node) Lookup(key string) ([]byte, bool) {
	n.Mutex.RLock()
	defer n.Mutex.RUnlock()
	val, ok := n.Store[key]
	return val, ok
}
