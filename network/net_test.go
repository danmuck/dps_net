package network

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/api/services/router"
	"github.com/danmuck/dps_net/config"
	"github.com/danmuck/dps_net/network/routing"

	// "github.com/danmuck/dps_net/network/routing"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// EchoService is a simple RPC service that echoes the RPC envelope.
type EchoService struct{}

func (e *EchoService) Echo(ctx context.Context, req *api.RPC) (*api.RPC, error) {
	return req, nil
}

func TestUDPInvokeEcho(t *testing.T) {
	// 1) Load config (auto-finds ../config/config.toml)
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}

	// 2) Bring up the manager (which starts the UDP server internally)
	contact := api.NewContact(make([]byte, api.KeyBytes), cfg.Address, strconv.Itoa(cfg.TCPPort), strconv.Itoa(cfg.UDPPort))
	mgr, err := NewNetworkManager(contact, *cfg)
	if err != nil {
		t.Fatalf("NewNetworkManager: %v", err)
	}
	mgr.Start()

	// 3) Seed the expected AppLock (must be 64 bytes → 128 hex chars)
	raw := strings.Repeat("0123456789abcdef", 8) // 16*8=128 hex chars
	lockBytes, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}
	var appLock api.AppLock
	copy(appLock[:], lockBytes)
	mgr.appLocks["EchoService"] = appLock

	// 4) Register our EchoService under the name "EchoService"
	desc := &grpc.ServiceDesc{
		ServiceName: "EchoService",
		Methods:     []grpc.MethodDesc{{MethodName: "Echo"}},
		Metadata:    lockBytes,
	}
	if err := mgr.RegisterService(desc, &EchoService{}); err != nil {
		t.Fatalf("RegisterService: %v", err)
	}

	// 5) Build a nested RPC envelope:
	//    inner := &api.RPC{ Payload: []byte("hello"), ... }
	//    outer := &api.RPC{ Payload: proto.Marshal(inner), ... }
	inner := &api.RPC{
		Service: "EchoService",
		Method:  "Echo",
		Sender:  api.NewContact(nil, "", "", ""),
		Payload: []byte("hello"),
	}
	innerBytes, err := proto.Marshal(inner)
	if err != nil {
		t.Fatalf("proto.Marshal(inner): %v", err)
	}
	outer := &api.RPC{
		Service: "EchoService",
		Method:  "Echo",
		Sender:  api.NewContact(nil, "", "", ""),
		Payload: innerBytes,
	}
	data, err := proto.Marshal(outer)
	if err != nil {
		t.Fatalf("proto.Marshal(outer): %v", err)
	}

	// 6) Send to the manager’s UDP endpoint
	client, err := net.ListenUDP("udp", nil)
	if err != nil {
		t.Fatalf("client ListenUDP: %v", err)
	}
	defer client.Close()

	addrStr := net.JoinHostPort(cfg.Address, strconv.Itoa(cfg.UDPPort))
	serverAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		t.Fatalf("ResolveUDPAddr %q: %v", addrStr, err)
	}
	if _, err := client.WriteToUDP(data, serverAddr); err != nil {
		t.Fatalf("WriteToUDP: %v", err)
	}

	// 7) Read back the outer reply
	buf := make([]byte, 1<<16)
	client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("ReadFromUDP: %v", err)
	}

	var gotOuter api.RPC
	if err := proto.Unmarshal(buf[:n], &gotOuter); err != nil {
		t.Fatalf("proto.Unmarshal outer: %v", err)
	}

	// 8) Unpack the inner envelope
	var gotInner api.RPC
	if err := proto.Unmarshal(gotOuter.Payload, &gotInner); err != nil {
		t.Fatalf("proto.Unmarshal inner: %v", err)
	}

	if string(gotInner.Payload) != "hello" {
		t.Errorf("expected inner payload %q, got %q",
			"hello", string(gotInner.Payload))
	}
}

// TestKademliaService_FindNode verifies that the FindNode RPC returns exactly
// the k closest contacts from the routing table.
func TestKademliaService_FindNode(t *testing.T) {
	randomNodes := 50
	// 1) Create a “local” contact (this node) and a routing table with k=3, alpha=3
	localID := api.GenerateRandomBytes(api.KeyBytes)
	local := api.NewContact(localID, "127.0.0.1", "0", "6669")
	svc := routing.NewRoutingTable(local, 3, 3)

	// 2) Define a deterministic target ID
	var targetID api.NodeID
	copy(targetID[:], []byte("fixed-target-123456")) // 16 bytes; rest zeros

	// 3) Seed the table with 5 random peers
	peers := make([]*api.Contact, randomNodes)
	for i := range randomNodes {
		id := api.GenerateRandomBytes(api.KeyBytes)
		addr := fmt.Sprintf("10.0.0.%d", i+1)
		udpPort := fmt.Sprintf("%d", 6670+i)
		peers[i] = api.NewContact(id, addr, "0", udpPort)
		// tell the routing table about each one
		if err := svc.Update(context.Background(), peers[i]); err != nil {
			t.Fatalf("svc.Update failed: %v", err)
		}
	}

	// 4) Call the RPC handler
	req := &router.FIND_NODE{
		From:     local,
		TargetId: targetID[:],
	}
	resp, err := svc.FindNode(context.Background(), req)
	if err != nil {
		t.Fatalf("FindNode RPC failed: %v", err)
	}

	// 5) Compute what the routing table itself would return
	want, err := svc.ClosestK(context.Background(), targetID)
	if err != nil {
		t.Fatalf("FindClosestK failed: %v", err)
	}

	// 6) Compare lengths
	if len(resp.Nodes) != len(want) {
		t.Fatalf("got %d nodes, want %d", len(resp.Nodes), len(want))
	}

	// 7) Compare each ID with bytes.Equal
	for i := range want {
		gotID := resp.Nodes[i].GetId()
		wantID := want[i].GetId()
		if !bytes.Equal(gotID, wantID) {
			t.Errorf("node %d: got ID %x, want %x", i, gotID, wantID)
		}
	}
	log.Printf("%v", svc.RoutingTableString())
}
