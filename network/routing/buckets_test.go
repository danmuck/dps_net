package routing

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/danmuck/dps_net/api"
)

// dummyNode implements api.Node just for ID().
type dummyNode struct{ id api.NodeID }

func (d dummyNode) Join(ctx context.Context, bootstrapAddr string) error { return nil }
func (d dummyNode) StoreValue(ctx context.Context, key api.NodeID, value []byte) error {
	return nil
}
func (d dummyNode) FindValue(ctx context.Context, key api.NodeID) ([]byte, []api.Contact, error) {
	return nil, nil, nil
}
func (d dummyNode) FindNode(ctx context.Context, key api.NodeID, count int) ([]api.Contact, error) {
	return nil, nil
}
func (d dummyNode) Shutdown(ctx context.Context) error { return nil }

// newNodeID makes a NodeID whose last byte is v (all other bytes zero).
func newNodeID(v byte) []byte {
	var id []byte = make([]byte, api.KeyBytes)
	id[len(id)-1] = v
	return id
}

func TestKBucket_InsertNew(t *testing.T) {
	// local := dummyNode{id: api.NodeID{}}

	local := api.NewContact(make([]byte, api.KeyBytes), "localhost", "6668", "6669")
	c1 := api.NewContact(newNodeID(1), "localhost", "6667", "6670")
	c2 := api.NewContact(newNodeID(2), "localhost", "6666", "6671")
	c3 := api.NewContact(newNodeID(3), "localhost", "6665", "6672")
	rt := &RoutingTable{local: local, k: 2, buckets: make([]*kBucket, 0)}
	b := newBucket(rt, 0)

	t.Run("first insert", func(t *testing.T) {
		b.Insert(c2)
		if got, want := len(b.peers), 1; got != want {
			t.Fatalf("len after first insert = %d; want %d", got, want)
		}
		if !api.SliceCompare(b.peers[0].GetId(), c2.GetId()) {
			// if got, want := b.peers[0].ID(), c2.ID(); got != want {
			t.Errorf("first element = %v; want %v", b.peers[0].GetId(), c2.GetId())
		}
	})

	t.Run("second insert and order", func(t *testing.T) {
		b.Insert(c1)
		if got, want := len(b.peers), 2; got != want {
			t.Fatalf("len after second insert = %d; want %d", got, want)
		}
		wantOrder := []api.NodeID{api.NodeID(c1.GetId()), api.NodeID(c2.GetId())}
		gotOrder := []api.NodeID{api.NodeID(b.peers[0].GetId()), api.NodeID(b.peers[1].GetId())}
		if !reflect.DeepEqual(gotOrder, wantOrder) {
			t.Errorf("order = %v; want %v", gotOrder, wantOrder)
		}
	})

	t.Run("duplicate insertion refreshes but keeps order", func(t *testing.T) {
		b.Insert(c1)
		if got, want := len(b.peers), 2; got != want {
			t.Fatalf("len after dup insert = %d; want %d", got, want)
		}
		if got, want := b.peers[0].ID(), c1.ID(); got != want {
			t.Errorf("first after dup = %v; want %v", got, want)
		}
	})

	t.Run("insertion over capacity evicts farthest", func(t *testing.T) {
		b.Insert(c3)
		if got, want := len(b.peers), 2; got != want {
			t.Fatalf("len after third insert = %d; want %d", got, want)
		}
		wantOrder := []api.NodeID{c1.ID(), c2.ID()} // c3 is farthest, so dropped
		gotOrder := []api.NodeID{b.peers[0].ID(), b.peers[1].ID()}
		if !reflect.DeepEqual(gotOrder, wantOrder) {
			t.Errorf("order after overcapacity = %v; want %v", gotOrder, wantOrder)
		}
	})
}
func TestKBucket_Insert(t *testing.T) {
	// local ID = 0x00…00
	local := api.NewContact(make([]byte, api.KeyBytes), "localhost", "6668", "6669")

	rt := &RoutingTable{local: local, k: 4, buckets: make([]*kBucket, 0)}

	b := newBucket(rt, 0)

	// Contacts at distances 1,2,3 (last‐byte values)
	c1 := api.NewContact(newNodeID(1), "localhost", "6667", "6670")
	c2 := api.NewContact(newNodeID(2), "localhost", "6666", "6671")
	c3 := api.NewContact(newNodeID(3), "localhost", "6665", "6672")

	// 1) Insert out of order
	b.Insert(c2)
	b.Insert(c1)
	wantOrder := []api.NodeID{c1.ID(), c2.ID()}
	got := []api.NodeID{b.peers[0].ID(), b.peers[1].ID()}
	if !reflect.DeepEqual(got, wantOrder) {
		t.Errorf("after inserting 2 then 1, order = %v; want %v", got, wantOrder)
	}

	// 2) Duplicate insertion should refresh but not change order or length
	b.Insert(c1)
	if len(b.peers) != 2 {
		t.Fatalf("after reinserting duplicate, len = %d; want 2", len(b.peers))
	}
	if b.peers[0].ID() != c1.ID() {
		t.Errorf("after duplicate insert, first = %v; want %v", b.peers[0].ID(), c1.ID())
	}

	// 3) Inserting a farther node should evict the worst when over capacity
	b.Insert(c3)
	if len(b.peers) != 3 {
		t.Fatalf("after inserting c3, len = %d; want 2", len(b.peers))
	}
	// c3 is farthest, so should be trimmed off
	wantOrder = []api.NodeID{c1.ID(), c2.ID()}
	got = []api.NodeID{b.peers[0].ID(), b.peers[1].ID()}
	if !reflect.DeepEqual(got, wantOrder) {
		t.Errorf("after inserting c3, order = %v; want %v", got, wantOrder)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////
// Insertion and Bucket Splitting
////////////////////////////////////////////////////////////////////////////////////////////////////////

// Helper: creates a NodeID with the given first-byte, others zero
func newIDFirstByte(b byte) []byte {
	var id []byte = make([]byte, api.KeyBytes)
	id[0] = b
	return id
}

func newIDLastByte(b byte) []byte {
	var id []byte = make([]byte, api.KeyBytes)
	id[len(id)-1] = b
	return id
}

func TestInsert_SortingAndCapacity(t *testing.T) {
	// local node has ID=0
	local := api.NewContact(make([]byte, api.KeyBytes), "localhost", "6668", "6669")
	rt := &RoutingTable{local: local, k: 2}
	b := newBucket(rt, 0)

	c1 := api.NewContact(newIDLastByte(1), "localhost", "6667", "6670")
	c2 := api.NewContact(newIDLastByte(2), "localhost", "6666", "6671")
	c3 := api.NewContact(newIDLastByte(3), "localhost", "6665", "6672")

	b.Insert(c3)
	b.Insert(c1)
	b.Insert(c2)

	// expect top-2 closest to 0: {1,2}
	want := []api.NodeID{c1.ID(), c2.ID()}
	got := []api.NodeID{b.peers[0].ID(), b.peers[1].ID()}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sorting+capacity: got %v, want %v", got, want)
	}
}

func TestDuplicateInsert_RefreshesPosition(t *testing.T) {
	local := api.NewContact(newIDLastByte(3), "localhost", "6668", "6669")
	rt := &RoutingTable{local: local, k: 3}
	b := newBucket(rt, 0)

	// contacts with IDs 3,1,2
	c1 := api.NewContact(newIDLastByte(1), "localhost", "6667", "6670")
	c2 := api.NewContact(newIDLastByte(2), "localhost", "6666", "6671")

	b.Insert(c1)
	b.Insert(c2)
	b.Insert(c1) // duplicate

	if len(b.peers) != 2 {
		t.Fatalf("duplicate insert: len = %d; want 2", len(b.peers))
	}
	if b.peers[0].ID() != c1.ID() {
		t.Errorf("duplicate insert: first = %v; want %v", b.peers[0].ID(), c1.ID())
	}
}

func TestSplit_PartitionsPeers(t *testing.T) {

	local := api.NewContact(make([]byte, api.KeyBytes), "localhost", "6668", "6669")
	rt := NewRoutingTable(local, 2, 1)

	// contacts with IDs 3,1,2
	// firstPeer has prefixLen(local, firstPeer)=0 → goes into bucket0
	firstPeer := api.NewContact(newIDFirstByte(0x80), "localhost", "6667", "6670")
	// secondPeer has prefixLen(local, secondPeer)=1 → should wind up in bucket1 after split
	secondPeer := api.NewContact(newIDFirstByte(0x40), "localhost", "6666", "6671")

	// 1) Fill bucket0 to capacity ([local, firstPeer])
	rt.Update(context.Background(), firstPeer)

	// 2) This insert sees bucket0 full, splits it, then inserts secondPeer into bucket1
	// which also contains the local node
	rt.Update(context.Background(), secondPeer)

	// After split, rt.buckets should be:
	//   buckets[0] (depth=0): contains firstPeer 			(prefixLen==0)
	//   buckets[1] (depth=1): contains secondPeer & local	(prefixLen>=1)
	if len(rt.buckets) != 2 {
		t.Fatalf("expected 2 buckets after split; got %d", len(rt.buckets))
	}

	left := rt.buckets[0]
	right := rt.buckets[1]

	// left must contain both local and firstPeer, but not secondPeer
	if !left.containsContact(firstPeer) {
		t.Errorf("left bucket missing firstPeer %v", firstPeer.ID())
	}
	if left.containsContact(secondPeer) {
		t.Errorf("left bucket should NOT contain secondPeer %v", secondPeer.ID())
	}

	// right must contain only secondPeer
	if !right.containsContact(local) {
		t.Errorf("left bucket missing local node")
	}
	if !right.containsContact(secondPeer) {
		t.Errorf("right bucket missing secondPeer %v", secondPeer.ID())
	}
	if right.containsContact(firstPeer) {
		t.Errorf("right bucket should NOT contain firstPeer %v", firstPeer.ID())
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////
// FindClosest
////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestFindClosest_SortsByDistance(t *testing.T) {
	local := api.NewContact(make([]byte, api.KeyBytes), "localhost", "6668", "6669")
	k := 3
	rt := NewRoutingTable(local, k, 1)

	ctx := context.Background()
	// Insert peers with IDs 5,1,3,2,4 (out of order)
	for i, v := range []byte{5, 1, 3, 2, 4} {
		rt.Update(ctx, api.NewContact(newIDLastByte(v), "localhost", strconv.Itoa(6667-i), strconv.Itoa(6670+i)))
	}

	// target=0 → distances are 1,2,3,4,5
	got, err := rt.FindClosestK(context.Background(), api.NodeID{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != k {
		t.Fatalf("len(got) = %d; \n\twant %d", len(got), k)
	}
	var gotIDs []api.NodeID
	for _, c := range got {
		gotIDs = append(gotIDs, c.ID())
	}
	want := []api.NodeID{api.NodeID(newIDLastByte(1)), api.NodeID(newIDLastByte(2)), api.NodeID(newIDLastByte(3))}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Errorf("FindClosest(0) = %v; \n\twant %v", gotIDs, want)
	}
}

// TestFindClosest_FewerPeersThanK verifies that when fewer than k peers exist,
// FindClosest returns them all, sorted.
func TestFindClosest_FewerPeersThanK(t *testing.T) {
	// local := dummyNode{id: newIDLastByte(0)}
	local := api.NewContact(make([]byte, api.KeyBytes), "localhost", "6668", "6669")
	k := 5
	rt := NewRoutingTable(local, k, 1)

	ctx := context.Background()
	// Insert only two peers
	for i, v := range []byte{2, 4} {
		rt.Update(ctx, api.NewContact(newIDLastByte(v), "localhost", strconv.Itoa(6667-i), strconv.Itoa(6670+i)))

	}

	got, err := rt.FindClosestK(context.Background(), api.NodeID{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d; \n\twant 2", len(got))
	}
	var gotLast []byte
	for _, c := range got {
		gotLast = append(gotLast, c.ID()[len(c.ID())-1])
	}
	want := []byte{2, 4}
	if !reflect.DeepEqual(gotLast, want) {
		t.Errorf("FindClosest(0) = %v; \n\twant %v", gotLast, want)
	}
}

// TestFindClosest_NonzeroTarget checks sorting when the target is not zero.
func TestFindClosest_NonzeroTarget(t *testing.T) {
	// local := dummyNode{id: newIDLastByte(0)}
	local := api.NewContact(make([]byte, api.KeyBytes), "localhost", "6668", "6669")

	k := 3
	rt := NewRoutingTable(local, k, 1)

	ctx := context.Background()
	// Insert peers 10,20,30
	for i, v := range []byte{10, 20, 30} {
		rt.Update(ctx, api.NewContact(newIDLastByte(v), "localhost", strconv.Itoa(6667-i), strconv.Itoa(6670+i)))

	}

	// target=25 → XOR distances: |10^25|=19, |20^25|=13, |30^25|=7
	got, err := rt.FindClosestK(context.Background(), api.NodeID(newIDLastByte(25)))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d; \n\twant 3", len(got))
	}
	var gotVals []byte
	for _, c := range got {
		gotVals = append(gotVals, c.ID()[len(c.ID())-1])
	}
	// Expect order {30,20,10}
	want := []byte{30, 20, 10}
	if !reflect.DeepEqual(gotVals, want) {
		t.Errorf("FindClosest(25) = %v; \n\twant %v", gotVals, want)
	}
}
