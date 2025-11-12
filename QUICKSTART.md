# Quick Start Guide

Get up and running with Durable Execution in 5 minutes!

## Prerequisites

- Go 1.21+
- Docker & Docker Compose
- protoc (Protocol Buffers compiler)

## Installation

```bash
# Install protoc (if not already installed)
# macOS:
brew install protobuf

# Linux:
sudo apt-get install -y protobuf-compiler

# Install Go tools
make install-tools

# Generate proto code
make generate

# Start NATS cluster
make docker-up
```

## Running the System

Open 3 terminals and run:

**Terminal 1:**
```bash
make run-service
```

**Terminal 2:**
```bash
make run-processor
```

**Terminal 3:**
```bash
make run-client
```

That's it! You just executed your first durable workflow! ✅

## Common Commands

```bash
# View all available commands
make help

# Build everything
make build

# Run tests
make test

# Submit a test command
SUBMIT_TEST_COMMAND=true make run-processor

# Stop NATS cluster
make docker-down

# Clean everything
make clean
```

## What Just Happened?

1. **NATS Cluster** started (3 nodes for HA)
2. **HelloService** started (gRPC server on port 9090)
3. **Processor** started (consuming commands from NATS)
4. **Client** submitted a workflow command
5. **Processor** executed the workflow durably
6. **State** was persisted to NATS KV

## Verify It Worked

```bash
# Check NATS health
curl http://localhost:8222/healthz

# View execution states (requires NATS CLI)
nats kv ls EXECUTION_STATE

# View stream info
nats stream info COMMANDS
```

## Next Steps

- Read [README.md](README.md) for architecture details
- See [USAGE_EXAMPLE.md](USAGE_EXAMPLE.md) for detailed examples
- Create custom workflows in `cmd/processor/main.go`
- Add new services by defining `.proto` files

## Troubleshooting

**Port already in use:**
```bash
# Change NATS ports in docker-compose.yml
```

**Can't connect to NATS:**
```bash
# Check if NATS is running
docker ps | grep nats

# View NATS logs
docker logs nats-1
```

**Proto generation fails:**
```bash
# Verify protoc is installed
protoc --version

# Reinstall tools
make install-tools
```

**Build fails:**
```bash
# Update dependencies
go mod download
go mod tidy

# Rebuild
make clean && make build
```

## Architecture at a Glance

```
┌─────────┐      ┌──────────────┐      ┌───────────┐
│ Client  │─────▶│ NATS Stream  │─────▶│ Processor │
└─────────┘      │  (Commands)  │      └─────┬─────┘
                 └──────────────┘            │
                                             │ Execute
                 ┌──────────────┐            │
                 │   NATS KV    │◀───────────┘
                 │   (State)    │      Journal Steps
                 └──────────────┘
                        ▲
                        │ Replay on Failure
                        │
                 ┌──────┴───────┐
                 │  Processor   │
                 │  (Restarted) │
                 └──────────────┘
```

## Key Features Demonstrated

✅ **Durable Execution**: Workflows survive failures
✅ **Deterministic Replay**: Steps are journaled and replayed
✅ **Exactly-Once**: Each invocation has unique ID
✅ **High Availability**: 3-node NATS cluster
✅ **State Persistence**: Execution state in NATS KV
✅ **gRPC Integration**: Type-safe service calls

Happy coding! 🚀
