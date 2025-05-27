package main

import (
	"crypto/rand"
	"math/big"
	"os"
	"strconv"

	"github.com/danmuck/dps_net/api"
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

func (c *Client) initSpin() error {
	id := api.GenerateRandomNodeID()
	if err := os.Setenv("NODE_ID", id.String()); err != nil {
		return LogErrorStr("error setting NODE_ID: %v", err)
	}

	udp := strconv.Itoa(peerUDP + len(c.peers))
	if err := os.Setenv("UDP_PORT", udp); err != nil {
		return LogErrorStr("error setting UDP_PORT: %v", err)
	}
	tcp := strconv.Itoa(peerTCP - len(c.peers))
	if err := os.Setenv("TCP_PORT", tcp); err != nil {
		return LogErrorStr("error setting TCP_PORT: %v", err)
	}

	// we assume this wont error because atm we dont care
	name, _ := randString(10)
	if err := os.Setenv("NODE_USER", name); err != nil {
		return LogErrorStr("error setting NODE_USER: %v", err)
	}
	return nil
}
