package services

import (
	context "context"
	"testing"

	api "github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/network/routing"
)

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

// TestKademliaService_FindNode_Deterministic verifies that the FindNode RPC
// returns exactly the k closest contacts when IDs differ only in the last byte.
func TestKademliaService_FindNode_Deterministic(t *testing.T) {
	// 1) Local contact and routing table (k=3, alpha=3)
	local := api.NewContact(newIDFirstByte(0x00), "127.0.0.1", "", "")
	rt := routing.NewRoutingTable(local, 3, 3)
	svc := NewKademliaService(rt)

	// 2) Deterministic target ID with last byte = 0x10
	target := newIDLastByte(0x10)

	// 3) Seed the table with 5 peers at various last-byte distances
	peerSpecs := []struct {
		id   []byte
		addr string
	}{
		{newIDLastByte(0x01), "10.0.0.1"},
		{newIDLastByte(0x02), "10.0.0.2"},
		{newIDLastByte(0x20), "10.0.0.3"},
		{newIDLastByte(0x03), "10.0.0.4"},
		{newIDLastByte(0xff), "10.0.0.5"},
	}
	for _, ps := range peerSpecs {
		c := api.NewContact(ps.id, ps.addr, "", "")
		if err := rt.Update(context.Background(), c); err != nil {
			t.Fatalf("rt.Update failed for %s: %v", ps.addr, err)
		}
	}

	// 4) Invoke the RPC handler
	req := &FIND_NODE{From: local, TargetId: target}
	resp, err := svc.FindNode(context.Background(), req)
	if err != nil {
		t.Fatalf("FindNode RPC failed: %v", err)
	}

	// 5) Expect exactly 3 closest: last bytes 0x01, 0x02, 0x03
	wantLastBytes := []byte{0x01, 0x02, 0x03}
	if len(resp.Nodes) != len(wantLastBytes) {
		t.Fatalf("got %d nodes, want %d", len(resp.Nodes), len(wantLastBytes))
	}
	for i, want := range wantLastBytes {
		got := resp.Nodes[i].GetId()[api.KeyBytes-1]
		if got != want {
			t.Errorf("node %d: got last-byte 0x%02x, want 0x%02x", i, got, want)
		}
	}
}

// TestKademliaService_FindNode_Deterministic_FirstByte verifies that
// the FindNode RPC returns exactly the k closest contacts when IDs differ
// only in the first byte.
func TestKademliaService_FindNode_Deterministic_FirstByte(t *testing.T) {
	// setup
	local := api.NewContact(newIDFirstByte(0x00), "127.0.0.1", "", "")
	rt := routing.NewRoutingTable(local, 3, 3)
	svc := NewKademliaService(rt)

	// deterministic target
	target := newIDFirstByte(0x10)

	// seed peers with various first-byte distances
	peerSpecs := []struct {
		id   []byte
		addr string
	}{
		{newIDFirstByte(0x01), "10.0.0.1"},
		{newIDFirstByte(0x02), "10.0.0.2"},
		{newIDFirstByte(0x20), "10.0.0.3"},
		{newIDFirstByte(0x03), "10.0.0.4"},
		{newIDFirstByte(0xff), "10.0.0.5"},
	}
	for _, ps := range peerSpecs {
		c := api.NewContact(ps.id, ps.addr, "", "")
		if err := rt.Update(context.Background(), c); err != nil {
			t.Fatalf("rt.Update failed for %s: %v", ps.addr, err)
		}
	}

	// invoke RPC
	req := &FIND_NODE{From: local, TargetId: target}
	resp, err := svc.FindNode(context.Background(), req)
	if err != nil {
		t.Fatalf("FindNode RPC failed: %v", err)
	}

	// expect 3 closest: first bytes 0x01, 0x02, 0x03
	wantFirst := []byte{0x01, 0x02, 0x03}
	if len(resp.Nodes) != len(wantFirst) {
		t.Fatalf("got %d nodes, want %d", len(resp.Nodes), len(wantFirst))
	}
	for i, w := range wantFirst {
		got := resp.Nodes[i].GetId()[0]
		if got != w {
			t.Errorf("node %d: got first-byte 0x%02x, want 0x%02x", i, got, w)
		}
	}
}
