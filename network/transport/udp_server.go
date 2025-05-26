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

	log.Printf("[NewUDPServer]: starting server on %s \n", u.address)

	// err = u.Start()
	// if err != nil {
	// 	return nil, err
	// }
	return u, nil
}

func (userv *UDPServer) Start() error {
	log.Printf("[userv.Start()] starting")
	ctx, cancel := context.WithCancel(context.Background())
	userv.cancel = cancel
	go func() {
		defer close(userv.recv) // channel closed exactly once, by this goroutine
		buf := make([]byte, 64<<10)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, peer, err := userv.conn.ReadFromUDP(buf)
				log.Printf("[userv.Start()] recv %d bytes from %s", n, peer)
				if err != nil {
					select {
					// check ctx.Done(), bail out if canceled
					case <-ctx.Done():
						return
					default:
						log.Println("[userv.Start()] udp read:", err)
						continue
					}
				}
				data := make([]byte, n)
				copy(data, buf[:n])

				// Unmarshal RPC
				var rpc api.RPC
				if err := proto.Unmarshal(data, &rpc); err != nil {
					log.Println("[userv.Start()] invalid rpc:", err)
					continue
				}
				log.Printf("[userv.Start()] parsed RPC – Service=%q Method=%q Sender=%v",
					rpc.Service, rpc.Method, rpc.Sender)

				// TODO: ?? verify Contact

				// Prepare the reply callback
				replyFn := func(res *api.RPC) error {
					out, err := proto.Marshal(res)
					if err != nil {
						return err
					}
					_, err = userv.conn.WriteToUDP(out, peer)
					return err
				}

				pkt := Packet{
					Ctx:     ctx,
					RPC:     &rpc,
					Peer:    rpc.Sender,
					Network: "udp",
					Reply:   replyFn,
				}
				userv.recv <- pkt // channel write is safe
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
