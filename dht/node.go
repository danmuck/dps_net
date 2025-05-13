package dht

import (
	"fmt"
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
	contact := Contact{ID: id, Address: address}
	node := &Node{
		ID:       id,
		Addr:     address,
		Info:     contact,
		Routing:  NewRoutingTable(contact, 10),
		Store:    make(map[string][]byte),
		LastSeen: time.Now(),
	}
	node.Info.LastSeen = node.LastSeen
	node.Conn = NewConnManager(node)

	err := node.Conn.Listen()
	if err != nil {
		panic(err)
	}

	return node
}

func (n *Node) Bootstrap(bootstrapAddr string) {
	msg := Message{
		Type: Ping,
		From: Contact{ID: n.ID, Address: n.Addr},
	}
	err := n.Conn.SendMessage(bootstrapAddr, msg)
	if err != nil {
		LogRPCFailure("Bootstrap ping failed: " + err.Error())
	} else {
		fmt.Println("Bootstrap ping sent to", bootstrapAddr)
	}
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
