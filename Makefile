.PHONY: help install-tools generate build docker-up docker-down run-processor run-service clean test

# Default target
help:
	@echo "Available targets:"
	@echo "  install-tools  - Install protoc and Go plugins"
	@echo "  generate       - Generate Go code from proto files"
	@echo "  build          - Build processor, services, client, and examples"
	@echo "  docker-up      - Start NATS JetStream cluster"
	@echo "  docker-down    - Stop NATS JetStream cluster"
	@echo "  run-processor  - Run the processor"
	@echo "  run-service    - Run the HelloService gRPC server"
	@echo "  run-client     - Run the full-featured client"
	@echo "  run-simple     - Run the simple 5-line example"
	@echo "  run-async      - Run the async example"
	@echo "  clean          - Clean generated files and binaries"
	@echo "  test           - Run tests"

# Install required tools
install-tools:
	@echo "Installing protoc-gen-go and protoc-gen-go-grpc..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Tools installed successfully"
	@echo ""
	@echo "Make sure you have protoc installed:"
	@echo "  macOS: brew install protobuf"
	@echo "  Linux: apt-get install -y protobuf-compiler"

# Generate Go code from proto files
generate:
	@echo "Generating Go code from proto files..."
	mkdir -p pb/hello pb/durable pb/ticket
	PATH="$(PATH):$(shell go env GOPATH)/bin" protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/hello.proto
	PATH="$(PATH):$(shell go env GOPATH)/bin" protoc --go_out=. --go_opt=paths=source_relative \
		proto/durable.proto
	PATH="$(PATH):$(shell go env GOPATH)/bin" protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/ticket.proto
	@mv proto/*.pb.go pb/ 2>/dev/null || true
	@mv pb/hello.pb.go pb/hello/ 2>/dev/null || true
	@mv pb/hello_grpc.pb.go pb/hello/ 2>/dev/null || true
	@mv pb/durable.pb.go pb/durable/ 2>/dev/null || true
	@mv pb/ticket.pb.go pb/ticket/ 2>/dev/null || true
	@mv pb/ticket_grpc.pb.go pb/ticket/ 2>/dev/null || true
	@echo "Code generation complete"

# Build binaries
build: generate
	@echo "Building processor..."
	go build -o bin/processor ./cmd/processor
	@echo "Building services..."
	go build -o bin/services ./cmd/services
	go build -o bin/ticket-service ./cmd/ticket-service
	@echo "Building client..."
	go build -o bin/client ./cmd/client
	@echo "Building examples..."
	go build -o bin/simple ./examples/simple
	go build -o bin/async ./examples/async
	go build -o bin/typed ./examples/typed
	go build -o bin/local ./examples/local
	go build -o bin/restate-style ./examples/restate-style
	go build -o bin/multi-service ./examples/multi-service
	go build -o bin/concurrent-workflows ./examples/concurrent-workflows
	go build -o bin/retry-failures ./examples/retry-failures
	go build -o bin/pause-cancel ./examples/pause-cancel
	go build -o bin/query-api ./examples/query-api
	go build -o bin/ticket-cart ./examples/ticket-cart
	go build -o bin/delayed-execution ./examples/delayed-execution
	go build -o bin/approval-workflow ./examples/approval-workflow
	go build -o bin/rest-invocation ./examples/rest-invocation
	go build -o bin/direct-grpc-call ./examples/direct-grpc-call
	@echo "Build complete"

# Start NATS JetStream cluster
docker-up:
	@echo "Starting NATS JetStream cluster..."
	docker-compose up -d
	@echo "Waiting for NATS to be ready..."
	sleep 5
	@echo "NATS cluster is ready"
	@echo "NATS URLs:"
	@echo "  Node 1: nats://localhost:4322 (HTTP: http://localhost:8322)"
	@echo "  Node 2: nats://localhost:4323 (HTTP: http://localhost:8323)"
	@echo "  Node 3: nats://localhost:4324 (HTTP: http://localhost:8324)"

# Stop NATS JetStream cluster
docker-down:
	@echo "Stopping NATS JetStream cluster..."
	docker-compose down
	@echo "NATS cluster stopped"

# Run processor
run-processor: build
	@echo "Running processor..."
	./bin/processor

# Run HelloService gRPC server
run-service: build
	@echo "Running HelloService..."
	./bin/services

# Run client to submit a command
run-client: build
	@echo "Running client..."
	./bin/client

# Run simple example (5-line client)
run-simple: build
	@echo "Running simple example..."
	./bin/simple

# Run async example
run-async: build
	@echo "Running async example..."
	./bin/async

# Clean generated files and binaries
clean:
	@echo "Cleaning generated files and binaries..."
	rm -rf pb/
	rm -rf bin/
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies downloaded"

# Verify setup
verify: docker-up
	@echo "Verifying NATS connection..."
	@echo "Checking NATS node 1..."
	@curl -s http://localhost:8322/varz | grep -q "server_id" && echo "✓ Node 1 is up" || echo "✗ Node 1 is down"
	@curl -s http://localhost:8323/varz | grep -q "server_id" && echo "✓ Node 2 is up" || echo "✗ Node 2 is down"
	@curl -s http://localhost:8324/varz | grep -q "server_id" && echo "✓ Node 3 is up" || echo "✗ Node 3 is down"

# Run typed workflow example
run-typed: build
	@echo "Running typed workflow example..."
	./bin/typed

# Run local development mode example
run-local: build
	@echo "Running local development example..."
	./bin/local

# Run Restate-style example
run-restate: build
	@echo "Running Restate-style example..."
	./bin/restate-style

# Run multi-service invocation example
run-multi-service: build
	@echo "Running multi-service invocation example..."
	./bin/multi-service

# Run concurrent workflows example
run-concurrent: build
	@echo "Running concurrent workflows example..."
	./bin/concurrent-workflows

# Run retry failures example
run-retry: build
	@echo "Running retry failures example..."
	./bin/retry-failures

# Run pause/cancel example
run-pause: build
	@echo "Running pause/cancel example..."
	./bin/pause-cancel

# Run query API example
run-query: build
	@echo "Running query API example..."
	./bin/query-api

# Run ticket service
run-ticket-service: build
	@echo "Running TicketService..."
	./bin/ticket-service

# Run ticket cart example (Restate-style)
run-ticket-cart: build
	@echo "Running ticket cart example (Restate-style)..."
	./bin/ticket-cart

# Run delayed execution example
run-delayed: build
	@echo "Running delayed execution example..."
	./bin/delayed-execution

# Run approval workflow example (awakeables)
run-approval: build
	@echo "Running approval workflow example (awakeables)..."
	./bin/approval-workflow

# Run REST invocation example
run-rest: build
	@echo "Running REST invocation example..."
	./bin/rest-invocation

# Run direct gRPC call example
run-direct-grpc: build
	@echo "Running direct gRPC call example..."
	./bin/direct-grpc-call
