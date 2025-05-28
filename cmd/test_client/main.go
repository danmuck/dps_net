package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danmuck/dps_net/node"
)

const (
	peerCfg string = "config/peer.toml"
	peerTCP int    = 6668
	peerUDP int    = 6670
	letters        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type Command func(args []string) error

type Client struct {
	local     *node.Node
	bootstrap *node.Node
	peers     []*node.Node
	users     map[string]*node.Node
	commands  map[string]Command
}

func main() {
	client := NewClient()
	client.run()
}

func NewClient() *Client {
	initLogs()

	client := &Client{
		local:    nil,
		peers:    make([]*node.Node, 0),
		commands: make(map[string]Command),
	}
	var err error
	client.local, err = node.NewNode("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init local node: %v\n", err)
		os.Exit(1)
	}

	// register command handlers
	//
	// General
	client.commands["start"] = client.start
	client.commands["ping"] = client.ping
	client.commands["spin"] = client.spin
	client.commands["killp"] = client.killp
	client.commands["join"] = client.join
	client.commands["refresh"] = client.refresh
	client.commands["table"] = client.table
	client.commands["stats"] = client.netstat
	client.commands["help"] = client.help
	client.commands["exit"] = client.shutdown
	client.commands["quit"] = client.shutdown

	// Logging
	client.commands["logs"] = client.logsMenu

	fmt.Println("[Node] initialized (not running)")
	return client
}

func (c *Client) run() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Type 'help' for commands.")
	c.help(nil)
	for {
		// sleep to let things resolve before the prompt pops
		time.Sleep(100 * time.Millisecond)
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		parts := strings.Fields(scanner.Text())
		if len(parts) == 0 {
			continue
		}
		cmd, args := parts[0], parts[1:]
		if fn, ok := c.commands[cmd]; ok {
			err := fn(args)
			if err != nil {
				if fe, isFatal := err.(FatalErr); isFatal {
					if fe.Err != nil {
						fmt.Fprintf(os.Stderr, "fatal: %v\n", fe.Err)
						os.Exit(1)
					}
					// clean shutdown
					return
				}
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "unknown command %q, type help\n", cmd)
		}
	}
}
