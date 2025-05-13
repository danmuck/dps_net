package dht

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

const BucketSize = 20

type Contact struct {
	ID       NodeID
	Address  string
	LastSeen time.Time // used for eviction
}

type kBucket struct {
	Start NodeID
	End   NodeID
	Nodes []Contact
	mu    sync.Mutex
}

type RoutingTable struct {
	id      NodeID
	k       int
	size    int
	Buckets []*kBucket
	mu      sync.Mutex
}

// Initialize a new routing table for the [local] node
//
//	Caller: node.go/NewNode()
//
// //
func NewRoutingTable(local Contact, k int) *RoutingTable {
	min := NodeID{}
	max := MaxNodeID()
	rt := &RoutingTable{
		id:      local.ID,
		k:       k,
		size:    1,
		Buckets: []*kBucket{{Start: min, End: max}},
	}
	rt.AddContact(local)
	return rt
}

// Add new contact into the current bucket,
//
//	handle splitting buckets as necessary
//
//	Caller: connection.go/handleMessage()
//
// //
func (rt *RoutingTable) AddContact(contact Contact) {
	// distance := xorDistance(rt.id, contact.ID)
	bucket := rt.FindBucket(contact.ID)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	fmt.Printf("Trying to add %x to bucket [%x–%x)\n", contact.ID[:4], bucket.Start[:4], bucket.End[:4])

	// Already present?
	for _, n := range bucket.Nodes {
		if n.ID == contact.ID {
			// Update last seen
			n.LastSeen = time.Now()
			return
		}
	}
	contact.LastSeen = time.Now()

	if len(bucket.Nodes) < BucketSize {
		bucket.Nodes = append(bucket.Nodes, contact)
		return
	}

	// Bucket full: decide whether to split
	if rt.bucketContainsSelf(bucket) {
		rt.splitBucket(bucket)
		rt.AddContact(contact) // Retry
	} else {
		// Eviction/ping check placeholder
	}
}

func (rt *RoutingTable) bucketContainsSelf(bucket *kBucket) bool {
	return NodeInRange(rt.id, bucket.Start, bucket.End)
}

func (rt *RoutingTable) splitBucket(bucket *kBucket) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	mid := prefixMidpoint(bucket.Start, bucket.End)

	left := &kBucket{Start: bucket.Start, End: mid}
	right := &kBucket{Start: mid, End: bucket.End}

	for _, n := range bucket.Nodes {
		if NodeInRange(n.ID, left.Start, left.End) {
			left.Nodes = append(left.Nodes, n)
		} else {
			right.Nodes = append(right.Nodes, n)
		}
	}

	// Replace in place
	for i, b := range rt.Buckets {
		if b == bucket {
			rt.Buckets = append(rt.Buckets[:i], append([]*kBucket{left, right}, rt.Buckets[i+1:]...)...)
			break
		}
	}
	rt.size = len(rt.Buckets)
}

func (rt *RoutingTable) FindBucket(id NodeID) *kBucket {
	for _, b := range rt.Buckets {
		if NodeInRange(id, b.Start, b.End) {
			return b
		}
	}
	return rt.Buckets[len(rt.Buckets)-1] // fallback
}

func prefixMidpoint(start, end NodeID) NodeID {
	var mid NodeID
	carry := 0
	for i := len(start) - 1; i >= 0; i-- {
		s := int(start[i])
		e := int(end[i])
		sum := s + e + carry
		mid[i] = byte(sum / 2)
		carry = (s + e + carry) % 2 * 256
	}
	return mid
}

// CompareNodeIDs returns -1, 0, or 1 for a < b, a == b, a > b
func CompareNodeIDs(a, b NodeID) int {
	for i := range len(a) {
		if a[i] < b[i] {
			return -1
		} else if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// =========================
// Bucket Debug Utility
// =========================
func (rt *RoutingTable) PrintBuckets() {
	fmt.Printf("Routing Table (%d buckets):\n", rt.size)
	for i, b := range rt.Buckets {
		fmt.Printf("  Bucket %2d [%x - %x) — %d nodes\n",
			i, b.Start[:4], b.End[:4], len(b.Nodes))
		for _, n := range b.Nodes {
			fmt.Printf("    - %x @ %s\n", n.ID[:4], n.Address)
		}
	}
}

// =========================
// Closest Node Finder
// =========================
func (rt *RoutingTable) FindClosest(target NodeID, count int) []Contact {
	var all []Contact
	for _, bucket := range rt.Buckets {
		bucket.mu.Lock()
		all = append(all, bucket.Nodes...)
		bucket.mu.Unlock()
	}
	type nodeDist struct {
		Contact  Contact
		Distance [20]byte
	}
	var scored []nodeDist
	for _, n := range all {
		scored = append(scored, nodeDist{
			Contact:  n,
			Distance: xorDistance(target, n.ID),
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		return CompareNodeIDs(scored[i].Distance, scored[j].Distance) < 0
	})

	var closest []Contact
	for i := 0; i < len(scored) && i < count; i++ {
		closest = append(closest, scored[i].Contact)
	}
	return closest
}
