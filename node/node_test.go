package node

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// Test that two nodes can Ping and Join, end-to-end over UDP.
func TestTwoNodePingAndJoin(t *testing.T) {
	const (
		addr       = "127.0.0.1"
		udpA, udpB = "7200", "7201"
		// assume TOML defaults for everything else
		configPath = "../config/peer.toml"
	)

	// helper to set env and build+start a node on a given UDP port
	makeNode := func(port string) *Node {
		os.Setenv("UDP_PORT", port)
		os.Setenv("TCP_PORT", "0")
		os.Setenv("ADDRESS", addr)
		n, err := NewNode(configPath)
		if err != nil {
			t.Fatalf("NewNode(%s): %v", port, err)
		}
		if err := n.Start(); err != nil {
			t.Fatalf("Start(%s): %v", port, err)
		}
		return n
	}

	A := makeNode(udpA)
	defer A.Stop()

	B := makeNode(udpB)
	defer B.Stop()

	// give them a moment to bind
	time.Sleep(50 * time.Millisecond)

	bAddr := fmt.Sprintf("%s:%s", addr, udpB)

	// 1) A can Ping B
	if err := A.Ping(bAddr); err != nil {
		t.Fatalf("A.Ping(B) failed: %v", err)
	}

	// 2) A can Join via B
	if err := A.Join(bAddr); err != nil {
		t.Fatalf("A.Join(B) failed: %v", err)
	}

	// 3) A can still Ping B after Join
	if err := A.Ping(bAddr); err != nil {
		t.Fatalf("A.Ping(B) after Join failed: %v", err)
	}

	// (optional) B can Ping A too
	aAddr := fmt.Sprintf("%s:%s", addr, udpA)
	if err := B.Ping(aAddr); err != nil {
		t.Fatalf("B.Ping(A) failed: %v", err)
	}
}
