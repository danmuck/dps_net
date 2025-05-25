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

// ////
// Create new Contact from NodeID and address
// //
func NewContact(id api.NodeID, address string) *Contact {
	c := &Contact{
		id:       id,
		address:  address,
		lastSeen: time.Now(),
	}
	return c
}

// // ////
// // Convert Contact to an api.Contact ProtoBuffer
// // //
// func ContactToProto(c *Contact) *api.Contact {
// 	id, err := api.NodeIDToSlice(c.ID())
// 	if err != nil {
// 		return nil
// 	}
// 	return &api.Contact{
// 		Id:       id,
// 		Address:  c.Address(),
// 		LastSeen: timestamppb.New(c.LastSeen()),
// 	}
// }

// // ////
// // Convert api.Contact ProtBuffer to Contact
// // //
// func ContactFromProto(pb *api.Contact) *Contact {
// 	id := api.SliceToNodeID(pb.GetId())
// 	// if err != nil {
// 	// 	return nil
// 	// }
// 	return &Contact{
// 		id:       id,
// 		address:  pb.Address,
// 		lastSeen: pb.LastSeen.AsTime(),
// 	}
// }

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
