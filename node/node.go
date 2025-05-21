package node

import (
	"context"
	"sync"

	"github.com/danmuck/dps_net/api"
)

type Node struct {
	info    api.ContactInterface
	net     api.NetworkManagerInterface
	storage api.StorageInterface
	cache   api.StorageInterface

	lock sync.RWMutex
}

func (n *Node) ID() api.NodeID

func (n *Node) Address() string

func (n *Node) Join(ctx context.Context, bootstrapAddr string) error

func (n *Node) StoreValue(ctx context.Context, key api.NodeID, value []byte) error

func (n *Node) FindValue(ctx context.Context, key api.NodeID) (value []byte, closest []Contact, err error)

func (n *Node) FindNode(ctx context.Context, key api.NodeID, count int) ([]Contact, error)

func (n *Node) Shutdown(ctx context.Context) error
