package routing

import (
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/danmuck/dps_net/api"
)

type kBucket struct {
	router *RoutingTable  // routing table containing this kBucket
	depth  int            // depth of this kBucket in the routing table
	isFull bool           // whether this kBucket is full
	peers  []*api.Contact // peers in this kBucket

	lock sync.RWMutex
}

func newBucket(router *RoutingTable, depth int) *kBucket {
	return &kBucket{
		router: router,
		depth:  depth,
		isFull: false,
		peers:  make([]*api.Contact, 0, router.k),
	}
}

func (b *kBucket) PrintString() string {
	localID := api.SliceToNodeID(b.router.local.GetId())
	output := fmt.Sprintf("Bucket %d -> \n", b.depth)
	for _, p := range b.peers {
		peerID := api.SliceToNodeID(p.GetId())
		// if err != nil {
		// 	output += fmt.Sprintf("[ERROR]: %s \n", err)
		// 	continue
		// }
		output += fmt.Sprintf("\t: prefix %03d : %08b \n",
			api.SharedPrefixLength(localID, peerID),
			peerID,
		)
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

func (b *kBucket) containsContact(c *api.Contact) bool {
	for _, contact := range b.peers {
		if api.SliceCompare(contact.GetId(), c.GetId()) {
			return true
		}
	}
	return false
}

// ////
// Insert adds c to the bucket (removing any old entry),
// then sorts b.peers by increasing XOR‐distance to the local node.
// //
func (b *kBucket) Insert(c *api.Contact) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	// avoid duplicates
	for i, existing := range b.peers {
		if api.SliceCompare(existing.GetId(), c.GetId()) {
			b.peers = slices.Delete(b.peers, i, i+1)
			break
		}
	}

	b.peers = append(b.peers, c)
	localID := api.SliceToNodeID(b.router.local.GetId())

	// sort the slice by XOR-distance: closest first
	sort.Slice(b.peers, func(i, j int) bool {
		ni_id := api.SliceToNodeID(b.peers[i].GetId())
		nj_id := api.SliceToNodeID(b.peers[j].GetId())
		ni := api.XorDistance(ni_id, localID)
		nj := api.XorDistance(nj_id, localID)
		return api.LessDistance(ni, nj)
	})

	// remove excess nodes from the tail of the slice
	if len(b.peers) > b.router.k {
		b.peers = b.peers[:b.router.k]
	}
	b.updateFull()

	return b.containsContact(c)
}

func (b *kBucket) Remove(c *api.Contact) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	for i, node := range b.peers {
		if api.SliceCompare(node.GetId(), c.GetId()) {
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

	if !b.containsContact(b.router.local) {
		// returns right = nil, this is checked in routing.go:Update()
		return b, nil
	}
	localID := api.SliceToNodeID(b.router.local.GetId())
	// fmt.Fprintf(os.Stderr, "[bucket.Split()] Splitting Bucket: %v \n", b.PrintString())

	left := newBucket(b.router, b.depth)
	right := newBucket(b.router, b.depth+1)

	for _, c := range b.peers {
		id := api.SliceToNodeID(c.GetId())
		depth := api.SharedPrefixLength(localID, id)
		// fmt.Fprintf(os.Stderr, "[bucket.Split()] Depth: %v for %08b \n", depth, id)

		if depth == b.depth {
			// fmt.Fprintf(os.Stderr, "[bucket.Split()] Insert Left: %08b \n", id)
			// if peer belongs in the current bucket, place it there
			left.peers = append(left.peers, c)
		} else {
			// fmt.Fprintf(os.Stderr, "[bucket.Split()] Insert Right: %08b \n", id)
			// otherwise send it forward to find its natural bucket
			right.peers = append(right.peers, c)
		}
	}

	left.updateFull()
	right.updateFull()
	return left, right
}
