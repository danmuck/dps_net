package kdht

import (
	"sync"

	"github.com/danmuck/dps_net/api"
)

type kBucket struct {
	parent *RoutingTable  // routing table containing this kBucket
	depth  int            // depth of this kBucket in the routing table
	isFull bool           // whether this kBucket is full
	peers  []*api.Contact // peers in this kBucket

	lock sync.RWMutex
}

func bucketInit(parent *RoutingTable, depth int) *kBucket {
	return &kBucket{
		parent: parent,
		depth:  depth,
		isFull: false,
		peers:  make([]*api.Contact, 0, parent.k),
	}
}
