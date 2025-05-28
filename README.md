# dps_net P2P Network

A simple peer-to-peer (P2P) networking framework using Kademlia-style routing.

> **Note:** This is *not* yet a full DHT; it currently provides peer discovery and messaging. DHT storage/persistence will be added in a future release.

## Features

* **Peer discovery** via Kademlia *FindNode* and *Ping* RPCs
* Simple CLI client (`cmd/test_client`) to start, ping, join, and spin peers
* Docker support for running the client and isolated peer containers

## Requirements

* Go 1.24+
* macOS/Linux
* Docker & Docker Compose (if running in container)

## Configuration

The node reads environment variables or a TOML file (`config/peer.toml`) for:

* `NODE_ID`: 160-bit (40 hex‐character) Kademlia node ID
* `UDP_PORT`: UDP port for RPCs
* `TCP_PORT`: (future) TCP port
* `NODE_USER`: human‐readable username

Example configs can be found in `config/`

## Running Locally

1. **Build** the client and nodes:

```bash

go build -o bin/client cmd/test\_client/\*.go

```

2. **Start a bootstrap peer** in one shell:

```bash

export NODE_ID=$(generate-40-hex)
export UDP_PORT=6670
export NODE_USER=bootstrap
./bin/node

```

3. **Run the client** in another shell:

```bash

export UDP\_PORT=6680
export NODE\_ID=\$(generate-40-hex)
export NODE\_USER=client
./bin/client

```

4. At the `> ` prompt, use commands:
   - `start` — start networking (UDP server)
   - `spin`  — launch an in-process dummy peer
   - `join localhost:6670` — bootstrap onto the network
   - `ping localhost:6670` — test connectivity

## Docker (Linux/macOS)

1. **Build the client container**:

```bash

docker-compose build client

````

2. **Run the client**:

```bash

docker-compose run --rm&#x20;
-e NODE\_ID=abcdef...01&#x20;
-e UDP\_PORT=6680&#x20;
client

```

3. **Spin peers** (each in its own container):

```bash
HOST_IP=$(ifconfig en0 | grep inet | awk '{print $2}')

# Launch peer1:
docker run -d --name peer1 \
  -e NODE_ID=peer1id... \
  -e UDP_PORT=6671 \
  -e NODE_USER=peer1 \
  client

# Launch peer2:
docker run -d --name peer2 \
  -e NODE_ID=peer2id... \
  -e UDP_PORT=6672 \
  -e NODE_USER=peer2 \
  client
````

Inside your client prompt, `join <HOST_IP>:6670` will bootstrap new peers.

## Future Work

* **DHT storage**: key/value persistence and retrieval
* **TCP transport**: optional reliable messaging
* **Security layers**: authentication & encryption

---

### Compile protobufs if necessary (they are included)

```bash
	protoc \
	  -I. \
	  --go_out=paths=source_relative:. \
	  --go-grpc_out=paths=source_relative:. \
	  api/rpc.proto \
	  network/services/kad_rpc.proto

```
