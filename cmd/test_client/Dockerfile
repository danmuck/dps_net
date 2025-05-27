# Stage 1: compile the Go CLI
FROM golang:1.24-alpine AS builder
WORKDIR /go/src/app

# Only copy go.mod/sum and download deps first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of your sources
COPY . .

# Build the entire test_client package
RUN CGO_ENABLED=0 GOOS=linux go build \
	-o /go/bin/client \
	./cmd/test_client

# Stage 2: a minimal runtime image
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app

# Copy just the compiled client binary and your config files
COPY --from=builder /go/bin/client .
COPY config ./config

# Expose whatever ports you need
EXPOSE 6669/udp
EXPOSE 6669/tcp

# Run the interactive client
ENTRYPOINT ["./client"]
