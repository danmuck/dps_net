package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/network/routing"
	// "github.com/danmuck/dps_net/network/routing"
	"google.golang.org/protobuf/proto"
)

func (nm *NetworkManager) updatePeer(ctx context.Context, peer *api.Contact) {
	// right after proto.Unmarshal(buf, &replyEnv):
	nm.router.Update(ctx, peer)
	nm.peers[peer.Username] = peer
	nm.active[peer] = true
}

// InvokeRPC sends a single RPC over UDP and waits for a reply.
func (m *NetworkManager) InvokeRPC(
	ctx context.Context,
	peerAddr string,
	service, method string,
	req, resp proto.Message,
) error {
	log.Printf("[NetworkManager]@%v Invoking RPC on %v", m.info.Username, peerAddr)

	// 1) Marshal the typed request
	payload, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal %s.%s: %w", service, method, err)
	}

	// 2) Wrap in your generic envelope
	envelope := &api.RPC{
		Service: service,
		Method:  method,
		Sender:  m.info,  // local node info
		Payload: payload, // actual typed RPC for the service
	}
	data, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	// 3) Dial and send over UDP
	udpAddr, err := net.ResolveUDPAddr("udp", peerAddr)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", peerAddr, err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("dial UDP %s: %w", peerAddr, err)
	}
	defer conn.Close()

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("send to %s: %w", peerAddr, err)
	}

	// 4) Read response with a deadline
	buf := make([]byte, 64<<10)
	deadline := time.Now().Add(3 * time.Second)
	conn.SetReadDeadline(deadline)

	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return fmt.Errorf("read from %s: %w", peerAddr, err)
	}

	// 5) Unmarshal the envelope
	var replyEnv api.RPC
	if err := proto.Unmarshal(buf[:n], &replyEnv); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	// 6) Finally unmarshal into the typed resp
	if err := proto.Unmarshal(replyEnv.Payload, resp); err != nil {
		return fmt.Errorf("unmarshal %s.%s response: %w", service, method, err)
	}

	// handle core routing e.g. Kademlia routing
	if service == "routing.KademliaService" {
		switch method {
		case "Ping":
			ack, ok := resp.(*routing.ACK)
			if !ok {
				return fmt.Errorf("expected *routing.ACK, got %T", resp)
			}
			m.lock.Lock()
			defer m.lock.Unlock()
			// record the Contact you just pinged
			// we already know its network address is peerAddr
			peer := ack.GetFrom()
			m.peers[peer.Username] = peer
			m.active[peer] = true
			// since we received an ack, add the peer to the routing table
			m.router.Update(ctx, peer)

			log.Printf("[NetworkManager]@%v got %v.Ack from %v",
				m.info.Username, service, ack.From.Username)
			log.Println(m.router.RoutingTableString())

		case "FindNode":
			nodes, ok := resp.(*routing.NODES)
			if !ok {
				return fmt.Errorf("expected *routing.NODES, got %T", resp)
			}
			log.Printf("[NetworkManager]@%v got %v.Nodes from %v",
				m.info.Username, service, nodes.From.Username)

			m.lock.Lock()
			defer m.lock.Unlock()
			for i := range nodes.Nodes {
				peer := nodes.Nodes[i]
				m.peers[peer.GetUsername()] = peer
				m.active[peer] = true
				m.router.Update(ctx, peer)
			}
		}
	}

	return nil
}
