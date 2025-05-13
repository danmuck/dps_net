package dht

import (
	"sort"
	"sync"
)

const BucketSize = 20

type Contact struct {
	ID      NodeID
	Address string
}

type KBucket struct {
	Nodes []Contact
	mu    sync.Mutex
}

type RoutingTable struct {
	SelfID  NodeID
	Buckets [160]*KBucket
}

func NewRoutingTable(selfID NodeID) *RoutingTable {
	rt := &RoutingTable{SelfID: selfID}
	for i := range 160 {
		rt.Buckets[i] = &KBucket{}
	}
	return rt
}

func bucketIndex(a, b NodeID) int {
	for i := range len(a) {
		x := a[i] ^ b[i]
		if x != 0 {
			for j := range 8 {
				if x&(0x80>>j) != 0 {
					return i*8 + j
				}
			}
		}
	}
	return 159
}

func (rt *RoutingTable) AddContact(contact Contact) {
	idx := bucketIndex(rt.SelfID, contact.ID)
	bucket := rt.Buckets[idx]
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	for _, n := range bucket.Nodes {
		if n.ID == contact.ID {
			return
		}
	}

	if len(bucket.Nodes) < BucketSize {
		bucket.Nodes = append(bucket.Nodes, contact)
		LogRoutingTableSize(rt)
	} else {
		// Optional: ping and evict logic here
	}
}

func xorDistance(a, b NodeID) [20]byte {
	var dist [20]byte
	for i := range 20 {
		dist[i] = a[i] ^ b[i]
	}
	return dist
}

type contactDistance struct {
	Contact  Contact
	Distance [20]byte
}

func (rt *RoutingTable) FindClosest(target NodeID, count int) []Contact {
	var all []contactDistance
	for _, bucket := range rt.Buckets {
		bucket.mu.Lock()
		for _, n := range bucket.Nodes {
			all = append(all, contactDistance{
				Contact:  n,
				Distance: xorDistance(target, n.ID),
			})
		}
		bucket.mu.Unlock()
	}

	sort.Slice(all, func(i, j int) bool {
		return string(all[i].Distance[:]) < string(all[j].Distance[:])
	})

	var closest []Contact
	for i := 0; i < len(all) && i < count; i++ {
		closest = append(closest, all[i].Contact)
	}
	return closest
}
