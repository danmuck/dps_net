package kdht

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/danmuck/dps_net/api"
)

// the RoutingTable represents the kademlia protocol routing table
type RoutingTable struct {
	local   api.Node   // the local node
	k       int        // k value as per kademlia
	alpha   int        // alpha value as per kademlia
	buckets []*kBucket // max length of api.KeyBits

	lock sync.Mutex
}

func NewRoutingTable(localNode api.Node, k int, alpha int) *RoutingTable {
	rt := &RoutingTable{
		local:   localNode,
		k:       k,
		alpha:   alpha,
		buckets: make([]*kBucket, 0, api.KeyBits),
	}
	rt.Update(context.TODO(), localNode.Contact())

	return rt
}

func (rt *RoutingTable) K() int {
	return rt.k
}

func (rt *RoutingTable) GetBucket(i int) ([]api.Contact, error) {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	if i < 0 || i >= len(rt.buckets) {
		return nil, fmt.Errorf("bucket index %d out of range", i)
	}
	// return a copy so caller can’t mutate your internal slice
	peers := make([]api.Contact, len(rt.buckets[i].peers))
	copy(peers, rt.buckets[i].peers)
	return peers, nil
}

func (rt *RoutingTable) Update(ctx context.Context, c api.Contact) error {
	for {
		rt.lock.Lock()

		bucketIndex := api.SharedPrefixLength(rt.local.ID(), c.ID())
		size := len(rt.buckets)
		fmt.Fprintf(os.Stderr, "[rt.Update(1)] Inserting-> index: %v size: %v \n", bucketIndex, size)

		if size == 0 || c.ID() == rt.local.ID() {
			bucket := newBucket(rt, 0)
			fmt.Fprintf(os.Stderr, "Inserting Local: %08b \n", c.ID())
			bucket.Insert(c)
			rt.buckets = append(rt.buckets, bucket)
			rt.lock.Unlock()
			return nil
		}

		if bucketIndex >= size {
			bucketIndex = max(size-1, 0)
		}

		bucket := rt.buckets[bucketIndex]
		fmt.Fprintf(os.Stderr, "[rt.Update(2)] Inserting-> index: %v size: %v \n", bucketIndex, size)
		// if this is the deepest bucket, we may need to split it
		if bucketIndex == size-1 && bucket.depth < api.KeyBits-1 && bucket.isFull {
			left, right := bucket.Split()
			if right == nil {
				rt.lock.Unlock()
				return fmt.Errorf("[no split] bucket does not contain the local node")
			}
			rt.buckets = append(rt.buckets[:bucketIndex], left, right)
			fmt.Fprintf(os.Stderr, "[rt.Update()] Left: %v Right: %v \n", left.peers, right.peers)
			rt.lock.Unlock()
			continue
		}

		rt.lock.Unlock()
		fmt.Fprintf(os.Stderr, "Inserting Other: %08b \n", c.ID())
		bucket.Insert(c)
		return nil
	}
}

func (rt *RoutingTable) Remove(ctx context.Context, c api.Contact) error {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	// get the bucket that the node belongs in
	bucketIndex := api.KBucketIndex(rt.local.ID(), c.ID())

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

func (rt *RoutingTable) FindClosestK(ctx context.Context, target api.NodeID) ([]api.Contact, error) {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	// gather all peers (except local) into a single slice
	all := make([]api.Contact, 0)
	distances := make(map[api.NodeID]api.NodeID, rt.k)
	for _, bucket := range rt.buckets {
		for _, c := range bucket.peers {
			if c.ID() == rt.local.ID() {
				continue
			}
			// NOTE: likely not necessary and extra overhead
			// checking for now in case there are issues later
			if _, seen := distances[c.ID()]; seen {
				continue
			}
			all = append(all, c)
			distances[c.ID()] = api.XorDistance(c.ID(), target)
		}
	}

	// sort globally by XOR-distance to target
	sort.Slice(all, func(i, j int) bool {
		return api.LessDistance(distances[all[i].ID()], distances[all[j].ID()])
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
	sb.WriteString(fmt.Sprintf("Routing table: [%08b]\n  %d buckets: \n", rt.local.ID(), len(rt.buckets)))
	for _, b := range rt.buckets {
		sb.WriteString(b.PrintString())
		// sb.WriteString(fmt.Sprintf("Bucket %2d (depth %2d): %d peers \n",
		// 	i, b.depth, len(b.peers)))
		// for _, c := range b.peers {
		// 	sb.WriteString(fmt.Sprintf("  - %x\n", c.ID()))
		// }
	}
	return sb.String()
}
