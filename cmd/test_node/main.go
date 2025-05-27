package main

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/node"
)

const (
	peerCfg string = "config/peer.toml"
	peerTCP int    = 6668
	peerUDP int    = 6670
	letters        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func randString(n int) (string, error) {
	b := make([]byte, n)
	// For each position, pick a random index into letters.
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		b[i] = letters[idx.Int64()]
	}
	return string(b), nil
}

func printHelp() {
	fmt.Println(`Available commands:
  start                   	Start networking (UDP server)
  ping <peerAddr>         	Ping a peer (e.g. "127.0.0.1:6669")
  spin						Spin up a dummy peer
  killp						Stop all dummy peers
  exit, quit              	Stop and exit this client
  help                    	Show this message`,
	)
}

func spinNode(udpMod, tcpMod int, bootstrapAddr string) (*node.Node, error) {
	nid := api.GenerateRandomNodeID()
	if err := os.Setenv("NODE_ID", nid.String()); err != nil {
		return nil, fmt.Errorf("error setting NODE_ID: %v", err)
	}

	udp := strconv.Itoa(peerUDP + udpMod)
	if err := os.Setenv("UDP_PORT", udp); err != nil {
		return nil, fmt.Errorf("error setting UDP_PORT: %v", err)
	}
	tcp := peerTCP - tcpMod
	if err := os.Setenv("TCP_PORT", strconv.Itoa(tcp)); err != nil {
		return nil, fmt.Errorf("error setting TCP_PORT: %v", err)
	}

	name, err := randString(10)
	if err := os.Setenv("NODE_USER", name); err != nil {
		return nil, fmt.Errorf("error setting NODE_USER: %v", err)
	}

	peer, err := node.NewNode("config/peer.toml")
	if err != nil {
		return nil, fmt.Errorf("Error initializing node: %v \n", err)
	}
	// var spinUDPcount, spinTCPcount int
	if err := peer.Start(); err != nil {
		return nil, fmt.Errorf("Failed to start node: %v \n", err)

	}
	target := bootstrapAddr + ":" + strconv.Itoa(peerUDP)
	fmt.Println("Node started and listening for RPCs.\n Bootstrapping ...")
	peer.Join(target)

	return peer, nil

}

func main() {
	var spinUDPcount, spinTCPcount int = 0, 0
	var spunNodes []*node.Node = make([]*node.Node, 0)

	// Initialize node and automatically start networking
	peer, err := node.NewNode("config/peer.toml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing node: %v \n", err)
		os.Exit(1)
	}
	if err := peer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start node: %v \n", err)
		os.Exit(1)
	}
	fmt.Println("Node started and listening for RPCs.")

	// Create a new Node (auto-loads config from config/config.toml)
	local, err := node.NewNode("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing node: %v \n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	printHelp()
	for {
		time.Sleep(500 * time.Millisecond)
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		cmd := strings.ToLower(fields[0])
		switch cmd {
		case "help":
			printHelp()

		case "start":
			if err := local.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start node: %v\n", err)
			} else {
				fmt.Println("Node started successfully.")
			}

		case "ping":
			if len(fields) < 2 {
				fmt.Println("Usage: ping <peerAddr>")
				continue
			}
			addr := fields[1]
			if err := local.Ping(addr); err != nil {
				fmt.Fprintf(os.Stderr, "Ping %s failed: %v\n", addr, err)
			} else {
				fmt.Printf("Ping to %s succeeded.\n", addr)
			}

		case "spin":
			spinUDPcount++
			spinTCPcount--
			n, err := spinNode(
				spinUDPcount,
				spinTCPcount,
				peer.Contact.Address,
			)
			if err != nil {
				fmt.Printf("Spin Node failed: %v", err)
			}
			spunNodes = append(spunNodes, n)

		case "killp":
			for i, p := range spunNodes {
				p.Stop()
				spunNodes[i] = nil
			}
			spunNodes = make([]*node.Node, 0)

		case "join":
			if len(fields) < 2 {
				fmt.Println("Usage: join <peerAddr>")
				continue
			}
			addr := fields[1]
			if err := local.Join(addr); err != nil {
				fmt.Fprintf(os.Stderr, "Join %s failed: %v\n", addr, err)
			} else {
				fmt.Printf("Join to %s succeeded.\n", addr)
			}

		case "exit", "quit":
			local.Stop()
			peer.Stop()
			for _, p := range spunNodes {
				p.Stop()
			}

			fmt.Println("Goodbye.")
			return

		default:
			fmt.Printf("Unknown command: %s (type help for list)", cmd)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
	}
}
