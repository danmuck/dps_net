package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/danmuck/dps_net/api"
)

type Config struct {
	NodeID         string        `toml:"node_id"`
	Address        string        `toml:"address"`
	K              int           `toml:"k"`
	Alpha          int           `toml:"alpha"`
	TCPPort        int           `toml:"tcp_port"`
	UDPPort        int           `toml:"udp_port"`
	BootstrapPeers []string      `toml:"bootstrap_peers"`
	Timeout        time.Duration `toml:"timeout"`
}

// Load reads configuration from a TOML file at path (if non-empty),
// sets defaults, and then overrides with environment variables.
func Load(path string) (*Config, error) {
	// Set defaults
	id := api.GenerateRandomNodeID()
	idstr := id.String()
	cfg := &Config{
		NodeID:  idstr,
		Address: api.GetOutboundIP(),
		K:       10,
		Alpha:   3,
		TCPPort: 6668,
		UDPPort: 6669,
		Timeout: 5 * time.Second,
	}

	// Load from TOML file if provided
	if path != "" {
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	// Environment overrides
	if v := os.Getenv("NODE_ID"); v != "" {
		cfg.NodeID = v
	}
	if v := os.Getenv("ADDRESS"); v != "" {
		cfg.Address = v
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
