package api

import (
	"context"
)

// type NodeInterface interface {
// 	// ID returns this node’s identifier.
// 	ID() NodeID

// 	// Address returns the network address (e.g. "ip:port") this node is listening on.
// 	Address() string

// 	Contact() Contact

// 	// Join bootstraps this node against a known peer.
// 	Join(ctx context.Context, bootstrapAddr string) error

// 	// StoreValue stores a (key,value) pair into the DHT.
// 	StoreValue(ctx context.Context, key NodeID, value []byte) error

// 	// FindValue looks up a key in the DHT; if not found, returns closest peers.
// 	FindValue(ctx context.Context, key NodeID) (value []byte, closest []Contact, err error)

// 	// FindNode returns up to count closest peers to target.
// 	FindNode(ctx context.Context, key NodeID, count int) ([]Contact, error)

// 	// Shutdown gracefully stops network listeners and flushes state.
// 	Shutdown(ctx context.Context) error
// }

type RoutingTableInterface interface {
	// K returns the configured bucket size (the “k” in k-buckets).
	K() int

	// Update inserts or refreshes a contact in the appropriate bucket.
	// If the bucket is full it’s up to the implementation whether to evict
	// the least-recently seen node or split the bucket (for Kademlia+ variants).
	Update(ctx context.Context, c *Contact) error

	// Remove deletes a contact from whatever bucket it lives in.
	// Useful if you detect a node has permanently failed.
	Remove(ctx context.Context, c *Contact) error

	// FindClosest returns up to count contacts closest (by XOR distance)
	// to the given target ID.
	ClosestK(ctx context.Context, target NodeID) ([]*Contact, error)

	// GetBucket returns all contacts currently in the bucket at index i.
	// The index usually corresponds to the length of the common-prefix with
	// the local node (i.e. the “distance range”).
	GetBucket(i int) ([]*Contact, error)
	GetBucketSize(i int) int

	// print the routing table to terminal
	RoutingTableString() string
}

// type Contact interface {
// 	// ID returns the peer’s NodeID.
// 	ID() NodeID
// 	// Address returns the peer’s network address.
// 	Address() string
// 	// LastSeen returns when we last heard from this peer.
// 	LastSeen() time.Time
// 	// Update LastSeen timestamp
// 	UpdateLastSeen()
// }

// type NetworkManagerInterface interface {
// 	// Ping a peer to check liveness.
// 	Ping(ctx context.Context, to Contact) error
// 	// FindNodeRPC asks a peer for nodes closest to target.
// 	FindNodeRPC(ctx context.Context, to Contact, target NodeID) ([]Contact, error)
// 	// FindValueRPC asks a peer for the value or closest peers.
// 	FindValueRPC(ctx context.Context, to Contact, key NodeID) (value []byte, closest []Contact, err error)
// 	// StoreRPC tells a peer to store (key,value).
// 	StoreRPC(ctx context.Context, to Contact, key NodeID, value []byte) error
// }

type StorageInterface interface {
	// Save persists the value under key.
	Save(ctx context.Context, key NodeID, value []byte) error
	// Find retrieves the value; found==false if missing.
	Find(ctx context.Context, key NodeID) (value []byte, found bool, err error)
}
