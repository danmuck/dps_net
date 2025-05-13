package dht

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type ConnManager struct {
	LocalAddr string
	NodeID    NodeID
	Node      *Node
}

func NewConnManager(node *Node) *ConnManager {
	return &ConnManager{LocalAddr: node.Addr, NodeID: node.ID, Node: node}
}

func (cm *ConnManager) SendMessage(target string, msg Message) error {
	udpAddr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	msg.From = cm.Node.Info

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	if err != nil {
		return err
	}

	LogRPC("SEND", msg, target)
	LogTrace(msg, "sent")
	return nil
}

func (cm *ConnManager) Listen() error {
	addr, err := net.ResolveUDPAddr("udp", cm.LocalAddr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	fmt.Println("Listening on:", conn.LocalAddr())

	go cm.handleIncomingMessages(conn)

	return nil
}

func (cm *ConnManager) handleIncomingMessages(conn *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		nBytes, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading UDP:", err)
			continue
		}

		var msg Message
		err = json.Unmarshal(buf[:nBytes], &msg)
		if err != nil {
			fmt.Println("Invalid message:", err)
			continue
		}
		LogRPC("RECV", msg, remoteAddr.String())
		LogTrace(msg, "received")
		cm.handleMessage(msg)
	}
}

func (cm *ConnManager) handleMessage(msg Message) {

	switch msg.Type {
	case Ping:
		reply := Message{Type: Pong, From: cm.Node.Info}
		cm.SendMessage(msg.From.Address, reply)
		cm.Node.Routing.AddContact(msg.From)

	case Pong:
		cm.Node.Routing.AddContact(msg.From)

	case Store:
		cm.Node.StoreValue(msg.Key, msg.Value)

	case FindNode:
		closest := cm.Node.Routing.FindBucket(msg.Target)
		reply := Message{Type: FoundNode, From: cm.Node.Info, Results: closest.Nodes}
		cm.SendMessage(msg.From.Address, reply)

	case FindValue:
		if val, ok := cm.Node.Lookup(msg.Key); ok {
			reply := Message{Type: FoundValue, From: cm.Node.Info, Key: msg.Key, Value: val}
			cm.SendMessage(msg.From.Address, reply)
		}
	}

	// cm.Node.Routing.AddContact(msg.From)
	LogRoutingTableSize(cm.Node.Routing)
}
