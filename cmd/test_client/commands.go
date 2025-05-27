package main

import (
	"fmt"
	"log"

	"github.com/danmuck/dps_net/node"
)

func (c *Client) help(args []string) error {
	fmt.Println(`Available commands:
  start                   	Start networking (UDP server)
  join <peerAddr>         	Join a peer network (e.g. "localhost:6669" or "10.0.5.50:6669")
  ping <peerAddr>         	Ping a peer (e.g. "localhost:6669" or "10.0.5.50:6669")
  spin						Spin up a dummy peer
  killp						Stop all dummy peers
  exit, quit              	Stop and exit this client
  help                    	Show this message`,
	)
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
	if err := c.local.Ping(addr); err != nil {
		return LogErrorStr("Ping %s failed: %v", addr, err)
	} else {
		fmt.Printf("Ping to %s succeeded.\n", addr)
	}
	return nil
}

func (c *Client) spin(args []string) error {
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
		return nil
	} else {
		// if this is not the bootstrap peer, bootstrap it
		fmt.Println("Node started and listening for RPCs.\n Bootstrapping ...")
		peer.Join(c.bootstrap.Contact.GetUDPAddress())
	}

	return nil
}

func (c *Client) killp(args []string) error {
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
	// os.Exit(0)
	return FatalErr{Err: nil}
}
