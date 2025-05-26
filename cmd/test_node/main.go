package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danmuck/dps_net/node"
)

func printHelp() {
	fmt.Println(`Available commands:
  start                   Start networking (UDP server)
  ping <peerAddr>         Ping a peer (e.g. "127.0.0.1:6669")
  exit, quit              Stop and exit this client
  help                    Show this message`,
	)
}

func main() {
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
			peer := fields[1]
			if err := local.Join(peer); err != nil {
				fmt.Fprintf(os.Stderr, "Ping %s failed: %v\n", peer, err)
			} else {
				fmt.Printf("Ping to %s succeeded.\n", peer)
			}

		case "exit", "quit":
			local.Stop()
			peer.Stop()
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
