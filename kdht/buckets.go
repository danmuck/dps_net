package kdht

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"sync"

	"github.com/danmuck/dps_net/api"
)

type kBucket struct {
	router *RoutingTable     // routing table containing this kBucket
	depth  int               // depth of this kBucket in the routing table
	isFull bool              // whether this kBucket is full
	peers  []api.ContactInterface // peers in this kBucket

	lock sync.RWMutex
}

func newBucket(router *RoutingTable, depth int) *kBucket {
	return &kBucket{
		router: router,
		depth:  depth,
		isFull: false,
		peers:  make([]api.ContactInterface, 0, router.k),
	}
}

func (b *kBucket) PrintString() string {
	output := fmt.Sprintf("Bucket %d -> \n", b.depth)
	for _, p := range b.peers {
		output += fmt.Sprintf("\t: index %d : prefix %d : %08b \n", api.KBucketIndex(b.router.local.ID(), p.ID()), api.SharedPrefixLength(b.router.local.ID(), p.ID()), p.ID())
	}
	return output
}

// ////
// Update kBucket-> isFull
// checks against b.router.k
// //
func (b *kBucket) updateFull() {
	if len(b.peers) < b.router.k {
		b.isFull = false
	} else {
		b.isFull = true
	}
}

func (b *kBucket) containsContact(c api.ContactInterface) bool {
	for _, contact := range b.peers {
		if contact.ID() == c.ID() {
			return true
		}
	}
	return false
}

// ////
// Insert adds c to the bucket (removing any old entry),
// then sorts b.peers by increasing XOR‐distance to the local node.
// //
func (b *kBucket) Insert(c api.ContactInterface) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	// avoid duplicates
	for i, existing := range b.peers {
		if existing.ID() == c.ID() {
			b.peers = slices.Delete(b.peers, i, i+1)
			break
		}
	}

	b.peers = append(b.peers, c)
	localID := b.router.local.ID()

	// sort the slice by XOR-distance: closest first
	sort.Slice(b.peers, func(i, j int) bool {
		ni := api.XorDistance(b.peers[i].ID(), localID)
		nj := api.XorDistance(b.peers[j].ID(), localID)
		return api.LessDistance(ni, nj)
	})

	// remove excess nodes from the tail of the slice
	if len(b.peers) > b.router.k {
		b.peers = b.peers[:b.router.k]
	}
	b.updateFull()

	return b.containsContact(c)
}

func (b *kBucket) Remove(c api.ContactInterface) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	for i, node := range b.peers {
		if node.ID() == c.ID() {
			// drop it out of the slice
			b.peers = slices.Delete(b.peers, i, i+1)
			b.updateFull()
			return true
		}
	}
	return false
}

func (b *kBucket) Split() (*kBucket, *kBucket) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if !b.containsContact(b.router.local.Contact()) {
		// returns right = nil, this is checked in routing.go:Update()
		return b, nil
	}
	fmt.Fprintf(os.Stderr, "[bucket.Split()] Splitting Bucket: %v \n", b.PrintString())

	left := newBucket(b.router, b.depth)
	right := newBucket(b.router, b.depth+1)

	for _, c := range b.peers {
		depth := api.SharedPrefixLength(b.router.local.ID(), c.ID())
		fmt.Fprintf(os.Stderr, "[bucket.Split()] Depth: %v for %08b \n", depth, c.ID())

		if depth == b.depth {
			fmt.Fprintf(os.Stderr, "[bucket.Split()] Insert Left: %08b \n", c.ID())
			// if peer belongs in the current bucket, place it there
			left.peers = append(left.peers, c)
		} else {
			fmt.Fprintf(os.Stderr, "[bucket.Split()] Insert Right: %08b \n", c.ID())
			// otherwise send it forward to find its natural bucket
			right.peers = append(right.peers, c)
		}
	}

	left.updateFull()
	right.updateFull()
	return left, right
}
