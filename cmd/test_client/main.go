package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/kdht"
)

// simpleContact implements both api.Node and api.Contact
// so it can serve as the local node and as peers.
type simpleContact struct{ id api.NodeID }

func (s simpleContact) ID() api.NodeID                                       { return s.id }
func (s simpleContact) Address() string                                      { return "" }
func (s simpleContact) LastSeen() time.Time                                  { return time.Now() }
func (s simpleContact) Contact() api.Contact                                 { return s }
func (s simpleContact) Join(ctx context.Context, bootstrapAddr string) error { return nil }
func (s simpleContact) StoreValue(ctx context.Context, key api.NodeID, value []byte) error {
	return nil
}
func (s simpleContact) FindValue(ctx context.Context, key api.NodeID) ([]byte, []api.Contact, error) {
	return nil, nil, nil
}
func (s simpleContact) FindNode(ctx context.Context, key api.NodeID, count int) ([]api.Contact, error) {
	return nil, nil
}
func (s simpleContact) Shutdown(ctx context.Context) error { return nil }

func main() {
	// Command-line flags
	num := flag.Int("n", 1000, "number of random nodes to insert")
	k := flag.Int("k", 20, "bucket size k")
	alpha := flag.Int("alpha", 3, "alpha concurrency value")
	flag.Parse()

	// Initialize routing table with random local node
	localID := api.GenerateRandomNodeID()
	local := simpleContact{id: localID}
	rt := kdht.NewRoutingTable(local, *k, *alpha)

	ctx := context.Background()
	// Seed the local node into the routing table
	// rt.Update(ctx, local.Contact())

	// Insert random contacts
	for _ = range *num {
		id := api.GenerateRandomNodeID()
		rt.Update(ctx, simpleContact{id: id}.Contact())
	}

	// Print the resulting routing table
	fmt.Print(rt.RoutingTableString())
}
