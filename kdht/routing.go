package kdht

import (
	"sync"

	"github.com/danmuck/dps_net/api"
)

// the RoutingTable represents the kademlia protocol routing table
type RoutingTable struct {
	local   *api.Node  // the local node
	k       int        // k value as per kademlia
	alpha   int        // alpha value as per kademlia
	size    int        // number of populated buckets in the table
	buckets []*kBucket // max length of api.KeyBits

	lock sync.Mutex
}
