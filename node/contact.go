package node

import (
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

func NewContact(id api.NodeID, address string) *Contact {
	c := &Contact{
		id:       id,
		address:  address,
		lastSeen: time.Now(),
	}
	return c
}

func (c *Contact) ID() api.NodeID {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.id
}

func (c *Contact) Address() string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.address
}

func (c *Contact) LastSeen() time.Time {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.lastSeen
}

func (c *Contact) UpdateLastSeen() int64 {
	c.lock.Lock()
	defer c.lock.Unlock()

	new := time.Now()
	window := new.UnixNano() - c.lastSeen.UnixNano()
	c.lastSeen = new

	return window
}
