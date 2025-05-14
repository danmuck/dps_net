package dht

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const BroadcastPort = 6669
const BroadcastInterval = 5 * time.Second

// BroadcastMessage contains the identity of a broadcasting peer
type BroadcastMessage struct {
	ID      NodeID
	Address string
}

// StartPeerDiscovery starts LAN-wide UDP broadcast to discover peers
func (n *Node) StartPeerDiscovery() {
	go n.broadcastPresence()
	go n.listenForBroadcasts()
}

func (n *Node) broadcastPresence() {
	addr := net.UDPAddr{IP: net.IPv4bcast, Port: BroadcastPort}
	conn, err := net.DialUDP("udp", nil, &addr)
	if err != nil {
		fmt.Println("Broadcast error:", err)
		return
	}
	defer conn.Close()

	msg := BroadcastMessage{ID: n.ID, Address: n.Addr}
	for {
		data, _ := json.Marshal(msg)
		conn.Write(data)
		time.Sleep(BroadcastInterval)
	}
}

func (n *Node) listenForBroadcasts() {
	addr := net.UDPAddr{IP: net.IPv4zero, Port: BroadcastPort}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		fmt.Println("Listen error:", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, 2048)
	for {
		nRead, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		var msg BroadcastMessage
		if err := json.Unmarshal(buf[:nRead], &msg); err != nil {
			continue
		}
		if msg.ID == n.ID {
			continue // skip self
		}
		fmt.Printf("Discovered peer %x @ %s\n", msg.ID[:4], msg.Address)
		n.Bootstrap(msg.Address)
	}
}
