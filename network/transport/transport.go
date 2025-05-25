package transport

import (
	"context"

	"github.com/danmuck/dps_net/api"
)

// Packet represents a single incoming RPC invocation.
type Packet struct {
	Ctx     context.Context      // request context
	RPC     *api.RPC             // the unmarshaled envelope
	Peer    *api.Contact         // e.g. "ip:port" or some nodeID string
	Network string               // tcp/udp
	Reply   func(*api.RPC) error // call this to send your response
}

// Dispatcher is what transport needs to invoke RPCs.
type Dispatcher interface {
	DispatchRPC(ctx context.Context, service, method string, payload []byte) ([]byte, error)
}

type Server interface {
	Start() error
	Stop()
	Receiver() <-chan Packet
}
