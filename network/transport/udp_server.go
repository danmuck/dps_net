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
	rcv     chan Packet
	cancel  context.CancelFunc
}

func NewUDPServer(addr, port string) (*UDPServer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr+":"+port)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	return &UDPServer{
		address: udpAddr,
		conn:    conn,
		rcv:     make(chan Packet),
	}, nil
}

func (s *UDPServer) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		defer close(s.rcv) // channel closed exactly once, by this goroutine
		buf := make([]byte, 64<<10)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, peer, err := s.conn.ReadFromUDP(buf)
				if err != nil {
					select {
					// check ctx.Done(), bail out if canceled
					case <-ctx.Done():
						close(s.rcv)
						return
					default:
						log.Println("udp read:", err)
						continue
					}
				}
				data := make([]byte, n)
				copy(data, buf[:n])

				// Unmarshal RPC
				var rpc api.RPC
				if err := proto.Unmarshal(data, &rpc); err != nil {
					log.Println("invalid rpc:", err)
					continue
				}

				// TODO: ?? verify Contact

				// Prepare the reply callback
				replyFn := func(res *api.RPC) error {
					out, err := proto.Marshal(res)
					if err != nil {
						return err
					}
					_, err = s.conn.WriteToUDP(out, peer)
					return err
				}

				pkt := Packet{
					Ctx:     ctx,
					RPC:     &rpc,
					Peer:    rpc.Sender,
					Network: "udp",
					Reply:   replyFn,
				}
				s.rcv <- pkt // channel write is safe
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
	return s.rcv
}
