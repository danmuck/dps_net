package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/danmuck/dps_net/api"
)

type Config struct {
	NodeID         string            `toml:"node_id"`
	Username       string            `toml:"username"`
	Address        string            `toml:"address"`
	K              int               `toml:"k"`
	Alpha          int               `toml:"alpha"`
	TCPPort        int               `toml:"tcp_port"`
	UDPPort        int               `toml:"udp_port"`
	BootstrapPeers []string          `toml:"bootstrap_peers"`
	Timeout        time.Duration     `toml:"timeout"`
	Refresh        time.Duration     `toml:"refresh"`
	AppLocks       map[string]string `toml:"app_locks"`
}

// Load reads configuration from a TOML file at path (if non-empty),
// sets defaults, and then overrides with environment variables.
func Load(path string) (*Config, error) {
	// Set defaults
	id := api.GenerateRandomNodeID()
	cfg := &Config{
		NodeID:         id.String(),
		Username:       "dirtpig",
		Address:        api.GetOutboundIP(),
		K:              10,
		Alpha:          3,
		TCPPort:        0,
		UDPPort:        6669,
		BootstrapPeers: make([]string, 0),
		Timeout:        5 * time.Second,
		Refresh:        30 * time.Second,
		AppLocks:       make(map[string]string),
	}

	// Auto-discover config file
	if path == "" {
		cands := []string{
			"config.toml",
			filepath.Join("config", "config.toml"),
			filepath.Join("..", "config", "config.toml"), // <— look one level up
		}
		for _, cand := range cands {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				path = cand
				break
			}
		}

		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	// fmt.Printf("\n Path: %s \nconfig: %+v \n", path, cfg)

	// Environment overrides
	if v := os.Getenv("NODE_ID"); v != "" {
		cfg.NodeID = v
	}
	if v := os.Getenv("ADDRESS"); v != "" {
		cfg.Address = v
	}
	if v := os.Getenv("NODE_USER"); v != "" {
		cfg.Username = v
	}
	if v := os.Getenv("TCP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.TCPPort = p
		}
	}
	if v := os.Getenv("UDP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.UDPPort = p
		}
	}
	if v := os.Getenv("BOOTSTRAP_PEERS"); v != "" {
		cfg.BootstrapPeers = strings.Split(v, ",")
	}
	if v := os.Getenv("TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeout = d
		}
	}

	return cfg, nil
}
