

```
	protoc \
	  -I. \
	  --go_out=paths=source_relative:. \
	  --go-grpc_out=paths=source_relative:. \
	  api/rpc.proto \
	  network/services/kad_rpc.proto

```
