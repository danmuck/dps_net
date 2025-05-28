package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/danmuck/dps_net/node"
)

func (c *Client) help(args []string) error {
	cmds := []struct{ name, desc string }{
		{"start", "Start networking (UDP server)"},
		{"join <peerAddr>", "Join a peer network (e.g. \"localhost:6669\")"},
		{"ping <peerAddr>", "Ping a peer (e.g. \"localhost:6669\")"},
		{"spin", "Spin up a dummy peer"},
		{"table <opt:index>", "Print a node’s routing table"},
		{"stats", "Print network stats"},
		{"killp", "Stop all dummy peers"},
		{"logs", "Open logs menu"},
		{"exit, quit", "Stop and exit this client"},
		{"help", "Show this message"},
	}

	// Compute the width of the longest command name
	maxName := 0
	for _, cmd := range cmds {
		if len(cmd.name) > maxName {
			maxName = len(cmd.name)
		}
	}

	fmt.Println()
	fmt.Println(" Available commands:")
	for _, cmd := range cmds {
		// %-*s left-justifies in a field of width maxName
		fmt.Printf("   %-*s  %s\n", maxName, cmd.name, cmd.desc)
	}
	fmt.Println()
	return nil
}

func (c *Client) start(args []string) error {
	if err := c.local.Start(); err != nil {
		return FatalError(err)
	} else {
		fmt.Println("Node started successfully.")
		return nil
	}
}

func (c *Client) join(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: join <peerAddr>")
		return LogErrorStr("bad join command")
	}
	addr := args[0]
	if len(addr) < 8 {
		tmp, err := strconv.Atoi(addr)
		if err == nil && tmp < len(c.peers) {
			addr = c.peers[tmp].Contact.GetUDPAddress()
		}
	}
	if err := c.local.Join(addr); err != nil {
		return LogErrorStr("Join %s failed: %v\n", addr, err)
	} else {
		fmt.Printf("Join to %s succeeded.\n", addr)
	}
	return nil
}

func (c *Client) ping(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: ping <peerAddr>")
		return LogErrorStr("bad ping command")
	}
	addr := args[0]
	if len(addr) < 8 {
		tmp, err := strconv.Atoi(addr)
		if err == nil && tmp < len(c.peers) {
			addr = c.peers[tmp].Contact.GetUDPAddress()
		}
	}
	if err := c.local.Ping(addr); err != nil {
		return LogErrorStr("Ping %s failed: %v", addr, err)
	} else {
		fmt.Printf("Ping to %s succeeded.\n", addr)
	}
	return nil
}

func (c *Client) spin(args []string) error {
	var count int
	if len(args) == 0 {
		count = 1
	} else {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return LogErrorStr("Usage: spin <opt:count> -> %v \n", err)
		}
		count = n
	}

	for range count {
		c.initSpin()
		peer, err := node.NewNode("config/peer.toml")
		if err != nil {
			return LogErrorStr("Error initializing node: %v \n", err)
		}
		if err := peer.Start(); err != nil {
			return LogErrorStr("Failed to start node: %v \n", err)
		}

		c.peers = append(c.peers, peer)

		// if this is the bootstrap peer, set it
		if len(c.peers) == 1 {
			c.bootstrap = peer
			continue
		} else {
			// if this is not the bootstrap peer, bootstrap it
			fmt.Println("Node started and listening for RPCs.\n Bootstrapping ...")
			peer.Join(c.bootstrap.Contact.GetUDPAddress())
		}
	}

	return nil
}

func (c *Client) killp(args []string) error {
	// kill specific peers
	if len(args) > 0 {
		for _, a := range args {
			n, err := strconv.Atoi(a)
			if err != nil || n >= len(c.peers) {
				continue
			}
			c.peers[n].Stop()
		}
		return nil
	}
	// otherwise kill all peers
	for i, p := range c.peers {
		p.Stop()
		c.peers[i] = nil
	}
	c.peers = make([]*node.Node, 0)
	return nil
}

func (c *Client) shutdown(args []string) error {
	// store state ??
	c.local.Stop()
	for _, p := range c.peers {
		p.Stop()
	}
	log.Println("goodbye!")
	return FatalErr{Err: nil}
}

func (c *Client) netstat(args []string) error {
	fmt.Printf(`

  Net Stats =>
	Local: %v:%v
	Bootstrap: %v
	Peers: %v
	Packets: %v

`,
		c.local.Contact.GetUsername(),
		c.local.Contact.GetUDPAddress(),
		c.bootstrap.Contact.GetUDPAddress(),
		len(c.peers),
		c.local.Stats().Packets)

	return nil
}

func (c *Client) table(args []string) error {
	if len(args) == 1 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return LogError(err)
		}
		if n >= len(c.peers) {
			return LogErrorStr("peer does not exist")
		}
		fmt.Printf("\n\nRouting Table for Node: %v:%v \n\n",
			c.peers[n].Contact.GetUsername(),
			c.peers[n].Contact.GetUDPAddress())
		c.peers[n].PrintRoutingTable()
		return nil
	}
	fmt.Printf("\n\nRouting Table for Node: %v:%v \n\n",
		c.local.Contact.GetUsername(),
		c.local.Contact.GetUDPAddress())
	c.local.PrintRoutingTable()
	return nil
}

func (c *Client) refresh(args []string) error {
	if len(args) == 1 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return LogError(err)
		}
		if n >= len(c.peers) {
			return LogErrorStr("peer does not exist")
		}
		c.peers[n].RefreshBuckets()
		fmt.Printf("buckets refreshed for peer[%v]", n)
		return nil
	}
	c.local.RefreshBuckets()
	fmt.Println("buckets refreshed for local")
	return nil
}
