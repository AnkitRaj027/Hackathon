.PHONY: all build test proto docker-build up down logs clean integration-test

BINARY_DIR := bin
PROTO_DIR := proto

all: proto build test

build:
	@echo "Building Vault binaries..."
	@mkdir -p $(BINARY_DIR)
	go build -o $(BINARY_DIR)/storage-node ./cmd/storage-node
	go build -o $(BINARY_DIR)/metadata ./cmd/metadata
	go build -o $(BINARY_DIR)/coordinator ./cmd/coordinator
	go build -o $(BINARY_DIR)/vaultctl ./cmd/vaultctl

test:
	@echo "Running unit and package tests..."
	go test -v -race ./internal/...

proto:
	@echo "Generating protobuf Go bindings..."
	protoc -I. --go_out=. --go_opt=module=vault --go-grpc_out=. --go-grpc_opt=module=vault proto/storage.proto proto/metadata.proto proto/coordinator.proto

docker-build:
	@echo "Building Docker images..."
	docker compose build

up:
	@echo "Starting Vault cluster..."
	docker compose up -d

down:
	@echo "Stopping Vault cluster..."
	docker compose down

logs:
	@echo "Showing cluster logs..."
	docker compose logs -f

clean:
	@echo "Cleaning binaries and artifacts..."
	rm -rf $(BINARY_DIR)
	rm -rf data/*
	rm -rf test-data/

integration-test:
	@echo "Running integration tests..."
	go test -v -race -timeout 5m ./tests/integration/...
