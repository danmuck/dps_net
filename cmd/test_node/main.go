package main

import (
	"bufio"
	"dps_net/dht"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <port>")
		return
	}
	ip := dht.GetOutboundIP()
	addr := ip + ":" + os.Args[1]

	node := dht.NewNode(addr)
	// err := node.Conn.Listen()
	// if err != nil {
	// 	panic(err)
	// }

	fmt.Println("Node ID:", hex.EncodeToString(node.ID[:]))
	fmt.Println("Listening on:", node.Addr)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		scanner.Scan()
		input := scanner.Text()
		args := strings.Split(input, " ")
		if len(args) == 0 {
			continue
		}
		switch args[0] {
		case "bootstrap":
			if len(args) < 2 {
				fmt.Println("Usage: bootstrap <ip:port>")
				continue
			}
			node.Bootstrap(args[1])

		case "ping":
			if len(args) < 2 {
				fmt.Println("Usage: ping <ip:port>")
				continue
			}
			target := args[1]
			msg := dht.Message{
				Type: dht.Ping,
				From: node.Info,
			}
			node.Conn.SendMessage(target, msg)
			// node.SendMessageTo(target, msg)
		case "store":
			if len(args) < 4 {
				fmt.Println("Usage: store <ip:port> <key> <value>")
				continue
			}
			target := args[1]
			msg := dht.Message{
				Type:  dht.Store,
				From:  node.Info,
				Key:   args[2],
				Value: []byte(args[3]),
			}
			node.Conn.SendMessage(target, msg)

		case "lookup":
			if len(args) < 3 {
				fmt.Println("Usage: lookup <ip:port> <key>")
				continue
			}
			target := args[1]
			msg := dht.Message{
				Type: dht.FindValue,
				From: node.Info,
				Key:  args[2],
			}
			node.Conn.SendMessage(target, msg)

		case "table":
			fmt.Println("Routing table:")
			for i, b := range node.Routing.Buckets {
				if len(b.Nodes) > 0 {
					fmt.Printf("Bucket %d [%d nodes]:\n", i, len(b.Nodes))
					for _, peer := range b.Nodes {
						fmt.Printf("  - %x @ %s\n", peer.ID[:4], peer.Address) // Short ID
					}
				}
			}
		}
	}
}
