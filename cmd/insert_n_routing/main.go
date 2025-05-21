package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/kdht"
)

// testContact implements both api.Node and api.Contact
// so it can serve as the local node and as peers.
type testContact struct {
	id   api.NodeID
	seen time.Time
}

func (s testContact) ID() api.NodeID      { return s.id }
func (s testContact) Address() string     { return "" }
func (s testContact) LastSeen() time.Time { return time.Now() }
func (s testContact) UpdateLastSeen()     { s.seen = time.Now() }

func (s testContact) Contact() api.ContactInterface                        { return s }
func (s testContact) Join(ctx context.Context, bootstrapAddr string) error { return nil }
func (s testContact) StoreValue(ctx context.Context, key api.NodeID, value []byte) error {
	return nil
}
func (s testContact) FindValue(ctx context.Context, key api.NodeID) ([]byte, []api.ContactInterface, error) {
	return nil, nil, nil
}
func (s testContact) FindNode(ctx context.Context, key api.NodeID, count int) ([]api.ContactInterface, error) {
	return nil, nil
}
func (s testContact) Shutdown(ctx context.Context) error { return nil }

func main() {
	// Command-line flags
	num := flag.Int("n", 1000, "number of random nodes to insert")
	k := flag.Int("k", 20, "bucket size k")
	alpha := flag.Int("alpha", 3, "alpha concurrency value")
	flag.Parse()

	// Initialize routing table with random local node
	localID := api.GenerateRandomNodeID()
	local := testContact{id: localID}
	rt := kdht.NewRoutingTable(local, *k, *alpha)

	ctx := context.Background()
	// Seed the local node into the routing table
	// rt.Update(ctx, local.Contact())

	// Insert random contacts
	for range *num {
		id := api.GenerateRandomNodeID()
		rt.Update(ctx, testContact{id: id}.Contact())
	}

	// Print the resulting routing table
	fmt.Print(rt.RoutingTableString())
}
