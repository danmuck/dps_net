package node

import (
	"context"
	"sync"
	"time"

	"github.com/danmuck/dps_net/api"
)

type Contact struct {
	id       api.NodeID // node ID of the peer
	address  string     // address of the peer
	lastSeen time.Time  // last time the peer was seen

	lock sync.RWMutex
}

func (c *Contact) ID() api.NodeID

func (c *Contact) Address() string

func (c *Contact) LastSeen() time.Time

type Node struct {
	info    api.Contact
	net     api.NetworkManager
	storage api.Storage
	cache   api.Storage

	lock sync.RWMutex
}

func (n *Node) ID() api.NodeID

func (n *Node) Address() string

func (n *Node) Join(ctx context.Context, bootstrapAddr string) error

func (n *Node) StoreValue(ctx context.Context, key api.NodeID, value []byte) error

func (n *Node) FindValue(ctx context.Context, key api.NodeID) (value []byte, closest []Contact, err error)

func (n *Node) FindNode(ctx context.Context, key api.NodeID, count int) ([]Contact, error)

func (n *Node) Shutdown(ctx context.Context) error
