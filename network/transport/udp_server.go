package transport

import (
	"context"
	"log"
	"net"

	"github.com/danmuck/dps_net/api"
	"google.golang.org/protobuf/proto"
)

type UDPServer struct {
	address *net.UDPAddr
	conn    *net.UDPConn
	recv    chan Packet
	cancel  context.CancelFunc
}

func NewUDPServer(addr, port string, recv chan Packet) (*UDPServer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr+":"+port)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	u := &UDPServer{
		address: udpAddr,
		conn:    conn,
		recv:    recv,
	}

	log.Printf("[NewUDPServer] %s \n", u.address)

	return u, nil
}

func (userv *UDPServer) Start() error {
	log.Printf("[UDPServer] (%v) starting", userv.address.String())
	ctx, cancel := context.WithCancel(context.Background())
	userv.cancel = cancel
	go func() {
		buf := make([]byte, 64<<10)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, peer, err := userv.conn.ReadFromUDP(buf)
				log.Printf("[UDPServer] (%v) recv %d bytes from %s", userv.address.String(), n, peer)
				if err != nil {
					select {
					// check ctx.Done(), bail out if canceled
					case <-ctx.Done():
						return
					default:
						log.Printf("[UDPServer] (%v) udp read: %v", userv.address.String(), err)
						continue
					}
				}
				data := make([]byte, n)
				copy(data, buf[:n])

				// Unmarshal RPC
				var rpc api.RPC
				if err := proto.Unmarshal(data, &rpc); err != nil {
					log.Printf("[UDPServer] (%v) invalid rpc: %v", userv.address.String(), err)
					continue
				}
				log.Printf("[UDPServer] (%v) parsed RPC – Service=%q Method=%q Sender=%v",
					userv.address.String(), rpc.Service, rpc.Method, rpc.Sender)

				// TODO: ?? verify Contact

				// Prepare the reply callback
				replyFn := func(res *api.RPC) error {
					out, err := proto.Marshal(res)
					if err != nil {
						return err
					}
					target := peer.IP.String() + ":" + rpc.Sender.GetUdpPort()
					log.Printf("[UDPServer] writing response to %v : %v.%v from: %v",
						target, res.Service, res.Method, res.Sender.Username)

					_, err = userv.conn.WriteToUDP(out, peer)
					return err
				}

				pkt := Packet{
					Ctx:     ctx,
					RPC:     &rpc,
					Sender:  rpc.Sender,
					Network: "udp",
					Reply:   replyFn,
				}
				log.Printf("[UDPServer] (%v) pushing packet onto network", userv.address.String())
				userv.recv <- pkt
			}
		}
	}()
	return nil
}

func (s *UDPServer) Stop() {
	s.cancel()
	s.conn.Close()
}

func (s *UDPServer) Receiver() <-chan Packet {
	return s.recv
}
