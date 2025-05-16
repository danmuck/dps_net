package node

import (
	"sync"
	"time"

	"github.com/danmuck/dps_net/api"
)

type Contact struct {
	id            api.NodeID // node ID of the peer
	address       string     // address of the peer
	naturalBucket uint8      // natural bucket of the peer
	lastSeen      time.Time  // last time the peer was seen

	lock sync.RWMutex
}

type Node struct {
	info    api.Contact
	net     api.NetworkManager
	storage LocalStorage
	cache   DataStorage

	lock sync.RWMutex
}
