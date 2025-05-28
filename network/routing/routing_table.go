package routing

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/danmuck/dps_net/api"
)

// the RoutingTable represents the kademlia protocol routing table
type RoutingTable struct {
	local   *api.Contact // the local node
	k       int          // k value as per kademlia
	alpha   int          // alpha value as per kademlia
	buckets []*kBucket   // max length of api.KeyBits

	lock sync.Mutex
}

func NewRoutingTable(localNode *api.Contact, k int, alpha int) *RoutingTable {
	rt := &RoutingTable{
		local:   localNode,
		k:       k,
		alpha:   alpha,
		buckets: make([]*kBucket, 0, api.KeyBits),
	}
	rt.Update(context.TODO(), localNode)

	return rt
}

func (rt *RoutingTable) GetLocal() *api.Contact {
	return rt.local
}

func (rt *RoutingTable) K() int {
	return rt.k
}

func (rt *RoutingTable) GetBucket(i int) ([]*api.Contact, error) {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	if i < 0 || i >= len(rt.buckets) {
		return nil, fmt.Errorf("bucket index %d out of range", i)
	}
	// return a copy so caller can’t mutate your internal slice
	peers := make([]*api.Contact, len(rt.buckets[i].peers))
	copy(peers, rt.buckets[i].peers)
	return peers, nil
}

func (rt *RoutingTable) GetBucketSize(i int) int {
	rt.lock.Lock()
	defer rt.lock.Unlock()
	if i >= len(rt.buckets) {
		return -1
	}
	return len(rt.buckets[i].peers)
}

func (rt *RoutingTable) Update(ctx context.Context, c *api.Contact) error {
	localID := api.SliceToNodeID(rt.local.GetId())
	otherID := api.SliceToNodeID(c.GetId())
	for {
		rt.lock.Lock()
		if len(rt.buckets) != 0 && bytes.Equal(localID[:], otherID[:]) {
			rt.lock.Unlock()
			return nil
		}

		bucketIndex := api.SharedPrefixLength(localID, otherID)
		size := len(rt.buckets)
		// log.Printf("[rt.Update(1)] Inserting-> index: %v size: %v \n", bucketIndex, size)
		if size == 0 && bytes.Equal(c.GetId(), rt.local.GetId()) {
			// if size == 0 || c.ID() == rt.local.ID() {
			bucket := newBucket(rt, 0)
			// log.Printf("[rt.Update()] inserting local: \n    %08b \n", otherID)
			bucket.Insert(c)
			rt.buckets = append(rt.buckets, bucket)
			rt.lock.Unlock()
			return nil
		}

		if bucketIndex >= size {
			bucketIndex = max(size-1, 0)
		}

		bucket := rt.buckets[bucketIndex]
		// log.Printf("[rt.Update(2)] Inserting-> index: %v size: %v \n", bucketIndex, size)
		// if this is the deepest bucket, we may need to split it
		if bucketIndex == size-1 && bucket.depth < api.KeyBits-1 && bucket.isFull {
			left, right := bucket.Split()
			if right == nil {
				rt.lock.Unlock()
				return fmt.Errorf("[no split] bucket does not contain the local node")
			}
			rt.buckets = append(rt.buckets[:bucketIndex], left, right)
			// log.Printf("[rt.Update()] Left: %v Right: %v \n", left.peers, right.peers)
			rt.lock.Unlock()
			continue
		}

		rt.lock.Unlock()
		// log.Printf("[rt.Update()] inserting other: \n    %08b \n", otherID)
		bucket.Insert(c)
		return nil
	}
}

func (rt *RoutingTable) Remove(ctx context.Context, c *api.Contact) error {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	localID := api.SliceToNodeID(rt.local.GetId())
	otherID := api.SliceToNodeID(c.GetId())

	// get the bucket that the node belongs in
	bucketIndex := api.KBucketIndex(localID, otherID)

	// if we dont have that bucket in our table, take the deepest bucket
	// if a node is not in its proper bucket, it will always be with the local node
	n := len(rt.buckets)
	if bucketIndex < 0 {
		bucketIndex = 0
	} else if bucketIndex >= n {
		bucketIndex = n - 1
	}

	rt.buckets[bucketIndex].Remove(c)
	return nil
}

func (rt *RoutingTable) ClosestK(ctx context.Context, target api.NodeID) ([]*api.Contact, error) {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	// gather all peers (except local) into a single slice
	all := make([]*api.Contact, 0)
	distances := make(map[api.NodeID]api.NodeID, rt.k)
	localID := api.SliceToNodeID(rt.local.GetId())
	for _, bucket := range rt.buckets {
		for _, c := range bucket.peers {
			otherID := api.SliceToNodeID(c.GetId())
			if api.SliceCompare(localID[:], otherID[:]) {
				continue
			}
			// NOTE: likely not necessary and extra overhead
			// checking for now in case there are issues later
			if _, seen := distances[otherID]; seen {
				continue
			}
			all = append(all, c)
			distances[otherID] = api.XorDistance(otherID, target)
		}
	}

	// sort globally by XOR-distance to target
	sort.Slice(all, func(i, j int) bool {
		ni_id := api.SliceToNodeID(all[i].GetId())
		nj_id := api.SliceToNodeID(all[j].GetId())
		return api.LessDistance(distances[ni_id], distances[nj_id])
	})

	// we only want to return k nodes
	if len(all) > rt.k {
		all = all[:rt.k]
	}
	return all, nil
}

// RoutingTableString returns a summary of the routing table's buckets.
func (rt *RoutingTable) RoutingTableString() string {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Routing table: [%08b]\n%d buckets: \n", rt.local.GetId(), len(rt.buckets)))
	for _, b := range rt.buckets {
		sb.WriteString(b.PrintString())
	}
	return sb.String()
}
