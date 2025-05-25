package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/network/routing"
	// "github.com/danmuck/dps_net/kdht"
)

// testNode implements both api.Node
// so it can serve as the local node
type testNode struct {
	id   api.NodeID
	seen time.Time
}

func (s testNode) Join(ctx context.Context, bootstrapAddr string) error { return nil }
func (s testNode) StoreValue(ctx context.Context, key api.NodeID, value []byte) error {
	return nil
}
func (s testNode) FindValue(ctx context.Context, key api.NodeID) ([]byte, []api.Contact, error) {
	return nil, nil, nil
}
func (s testNode) FindNode(ctx context.Context, key api.NodeID, count int) ([]api.Contact, error) {
	return nil, nil
}
func (s testNode) Shutdown(ctx context.Context) error { return nil }

func main() {
	// Command-line flags
	num := flag.Int("n", 1000, "number of random nodes to insert")
	k := flag.Int("k", 20, "bucket size k")
	alpha := flag.Int("alpha", 3, "alpha concurrency value")
	flag.Parse()

	// Initialize routing table with random local node
	localID := api.GenerateRandomNodeID()
	local := api.NewContact(localID[:], "localhost", "6668", "6669")

	rt := routing.NewRoutingTable(local, *k, *alpha)

	ctx := context.Background()
	// Seed the local node into the routing table
	// rt.Update(ctx, local.Contact())

	// Insert random contacts
	for i := range *num {
		id := api.GenerateRandomNodeID()
		rt.Update(ctx, api.NewContact(id[:], "localhost", strconv.Itoa(6667-i), strconv.Itoa(6670+i)))

	}

	// Print the resulting routing table
	fmt.Print(rt.RoutingTableString())
}
