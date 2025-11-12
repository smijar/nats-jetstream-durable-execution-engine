# Durable Execution Engine on NATS JetStream

A production-ready durable execution engine built on NATS JetStream in Go. This system provides deterministic workflow execution with exactly-once semantics, automatic replay, and fault tolerance.

## Quick Example (5 lines!)

```go
c, _ := client.NewClient("nats://127.0.0.1:4322")
defer c.Close()

result, _ := c.Invoke(context.Background(), "hello_workflow")
var resp hellopb.HelloResponse
result.GetFirstResponse(&resp)

fmt.Println(resp.Message)
// Output: Hello, World! Welcome to Durable Execution on NATS JetStream.
```

**See [SIMPLE_CLIENT.md](SIMPLE_CLIENT.md) for the simple SDK** or continue reading for the full architecture.

## Features

- **Durable Execution**: Workflows survive failures and can be resumed from the last successful step
- **Deterministic Replay**: Journal-based replay ensures consistent execution across retries
- **Exactly-Once Semantics**: Each invocation has a unique ID with idempotent step execution
- **Workflow Lifecycle Control**: Pause, resume, and cancel workflows with full state management
- **Failure Detection & Retry**: Identify failed workflows and selectively retry them
- **NATS JetStream**: Leverages JetStream for durable messaging and KV storage
- **gRPC/Protobuf**: Type-safe service contracts with protobuf
- **Distributed**: 3-node NATS cluster for high availability
- **Production Ready**: Comprehensive error handling, logging, and state management

## Architecture

### Core Components

1. **Command Log**: JetStream stream for durable, ordered, partitioned command storage
2. **State Store**: JetStream KV buckets for execution state persistence
3. **Processor**: Reads commands, replays journals, maintains exactly-once semantics
4. **Durable Context**: SDK for handlers to invoke services with automatic journaling
5. **gRPC Services**: Business logic services invoked durably

### Execution Flow

```
Client → Command Stream → Processor → Load State from KV
                                    ↓
                            Replay from Journal
                                    ↓
                            Execute Handler (durable.Context)
                                    ↓
                            Call gRPC Service
                                    ↓
                            Journal Result to KV
                                    ↓
                            Save Execution State
```

### Key Guarantees

- **Deterministic Replay**: On failure, execution replays from journal without re-executing completed steps
- **Atomic State Updates**: State changes are atomically persisted to KV
- **Idempotency**: Each step is idempotent and cached in the journal
- **Fault Tolerance**: Survives process crashes, network failures, and service restarts

## Project Structure

```
.
├── proto/                  # Protobuf definitions
│   ├── hello.proto        # HelloService: SayHello(name) -> message
│   └── durable.proto      # Command, JournalEntry, ExecutionState
├── pb/                    # Generated proto Go code
├── pkg/
│   ├── jetstream/         # JetStream client wrapper
│   ├── durable/           # SDK context for handlers
│   └── execution/         # Processor, replay logic, state management
├── cmd/
│   ├── processor/         # Main processor service
│   └── services/          # Example gRPC services
├── docker-compose.yml     # 3-node NATS JetStream cluster
├── nats.conf             # NATS configuration
├── go.mod                # Go module file
└── Makefile              # Build and run commands
```

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Protocol Buffers compiler (protoc)
- Make

### Installing protoc

**macOS:**
```bash
brew install protobuf
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get update
sudo apt-get install -y protobuf-compiler
```

**Verify installation:**
```bash
protoc --version
```

## Quick Start

### 1. Install Go tools

```bash
make install-tools
```

This installs:
- `protoc-gen-go`: Protobuf Go code generator
- `protoc-gen-go-grpc`: gRPC Go code generator

### 2. Generate protobuf code

```bash
make generate
```

### 3. Start NATS JetStream cluster

```bash
make docker-up
```

This starts a 3-node NATS cluster:
- Node 1: `nats://localhost:4222` (HTTP: `http://localhost:8222`)
- Node 2: `nats://localhost:4223` (HTTP: `http://localhost:8223`)
- Node 3: `nats://localhost:4224` (HTTP: `http://localhost:8224`)

### 4. Run the HelloService gRPC server

In a new terminal:
```bash
make run-service
```

The service listens on `localhost:9090`.

### 5. Run the processor

In another terminal:
```bash
make run-processor
```

The processor will:
1. Connect to NATS
2. Create the command stream and state KV bucket
3. Register workflow handlers
4. Start consuming commands

**Configure processor concurrency and timeouts:**
```bash
# Set max concurrent workflows (default: 100)
./bin/processor --max-concurrent 50

# Set workflow timeout before redelivery (default: 300s / 5min)
./bin/processor --ack-wait 600

# Combine both settings
./bin/processor --max-concurrent 200 --ack-wait 600
```

### 6. Verify the setup

```bash
make verify
```

## Usage

### End-to-End Example: Running Your First Workflow

Let's walk through a complete example of submitting a workflow and seeing it execute.

**Step 1: Start all services**

```bash
# Terminal 1: Start NATS
make docker-up

# Terminal 2: Start HelloService
make run-service

# Terminal 3: Start Processor
make run-processor
```

**Step 2: Submit a test command**

The processor can be configured to submit a test command on startup. Set the environment variable:

```bash
SUBMIT_TEST_COMMAND=true make run-processor
```

You'll see output like:
```
2024/11/10 22:00:00 Starting Durable Execution Processor...
2024/11/10 22:00:00 Setting up JetStream streams...
2024/11/10 22:00:00 Setting up KV buckets...
2024/11/10 22:00:01 Submitting test command: abc-123-def-456
2024/11/10 22:00:01 Processor started, consuming from processor-1
2024/11/10 22:00:01 Processing command: invocation_id=abc-123-def-456 handler=hello_workflow
2024/11/10 22:00:01 Executing hello workflow: invocation_id=abc-123-def-456
2024/11/10 22:00:01 Hello workflow completed: message=Hello, World! Welcome to Durable Execution on NATS JetStream.
2024/11/10 22:00:01 Execution completed successfully: abc-123-def-456
```

### Submitting Commands Programmatically

Create a simple client to submit commands:

**`cmd/client/main.go`:**
```go
package main

import (
    "context"
    "log"

    "github.com/google/uuid"
    "github.com/sanjaymijar/my-durable-execution/pb/durable"
    "github.com/sanjaymijar/my-durable-execution/pkg/execution"
    "github.com/sanjaymijar/my-durable-execution/pkg/jetstream"
)

func main() {
    // Connect to NATS
    jsClient, err := jetstream.NewClient("nats://localhost:4222")
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer jsClient.Close()

    // Create processor (for command submission)
    processor, err := execution.NewProcessor(jsClient)
    if err != nil {
        log.Fatalf("Failed to create processor: %v", err)
    }

    ctx := context.Background()

    // Submit a command
    cmd := &durable.Command{
        InvocationId: uuid.New().String(),
        Handler:      "hello_workflow",
        Service:      "HelloService",
        Args:         []byte{},
        PartitionKey: "user-123",
        Sequence:     1,
    }

    log.Printf("Submitting workflow: %s", cmd.InvocationId)
    if err := processor.SubmitCommand(ctx, cmd); err != nil {
        log.Fatalf("Failed to submit command: %v", err)
    }

    log.Println("Command submitted successfully!")
}
```

**Build and run:**
```bash
go build -o bin/client ./cmd/client
./bin/client
```

### Defining Custom Workflow Handlers

Workflow handlers receive a `durable.Context` and can make durable calls to gRPC services:

**Example: Multi-step workflow**
```go
func orderProcessingWorkflow(ctx *durable.Context) error {
    log.Printf("Starting order processing: invocation_id=%s", ctx.InvocationID())

    // Step 1: Validate order
    validateReq := &orderpb.ValidateRequest{OrderId: "order-123"}
    validateResp := &orderpb.ValidateResponse{}
    if err := ctx.DurableCall("OrderService", "Validate", validateReq, validateResp); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    // Step 2: Process payment
    paymentReq := &paymentpb.ChargeRequest{Amount: 99.99}
    paymentResp := &paymentpb.ChargeResponse{}
    if err := ctx.DurableCall("PaymentService", "Charge", paymentReq, paymentResp); err != nil {
        return fmt.Errorf("payment failed: %w", err)
    }

    // Step 3: Ship order
    shipReq := &shippingpb.ShipRequest{OrderId: "order-123"}
    shipResp := &shippingpb.ShipResponse{}
    if err := ctx.DurableCall("ShippingService", "Ship", shipReq, shipResp); err != nil {
        return fmt.Errorf("shipping failed: %w", err)
    }

    log.Printf("Order processing completed: tracking=%s", shipResp.TrackingNumber)
    return nil
}
```

**Key benefits:**
- If the workflow fails at step 2, on retry it will replay steps 1-2 from the journal and only re-execute step 3
- Each step is journaled with its request and response
- State is persisted after each step
- The workflow is deterministic across retries

### Registering Handlers

In your processor main (`cmd/processor/main.go`):

```go
func main() {
    // ... setup code ...

    // Create processor
    processor, err := execution.NewProcessor(jsClient)
    if err != nil {
        log.Fatalf("Failed to create processor: %v", err)
    }

    // Register workflow handlers
    processor.RegisterHandler("hello_workflow", helloWorkflowHandler)
    processor.RegisterHandler("order_processing", orderProcessingWorkflow)
    processor.RegisterHandler("payment_refund", paymentRefundWorkflow)

    // Start processor
    processor.Start(ctx, "processor-1")
}
```

### Viewing Execution State

You can inspect execution state using the NATS CLI or by querying the KV bucket directly.

**Using NATS CLI:**
```bash
# Install NATS CLI
go install github.com/nats-io/natscli/nats@latest

# List all execution states
nats kv ls EXECUTION_STATE

# Get specific execution state
nats kv get EXECUTION_STATE <invocation-id>

# Watch for state changes
nats kv watch EXECUTION_STATE
```

**Programmatically querying state:**
```go
func getExecutionState(jsClient *jetstream.Client, invocationID string) (*durable.ExecutionState, error) {
    ctx := context.Background()
    kv, err := jsClient.GetStateKV(ctx)
    if err != nil {
        return nil, err
    }

    entry, err := kv.Get(ctx, invocationID)
    if err != nil {
        return nil, err
    }

    var state durable.ExecutionState
    if err := proto.Unmarshal(entry.Value(), &state); err != nil {
        return nil, err
    }

    return &state, nil
}
```

### Testing Workflows

**Unit testing a workflow handler:**
```go
func TestOrderProcessingWorkflow(t *testing.T) {
    // Mock journal for replay
    journal := []*durable.JournalEntry{
        {
            StepNumber: 0,
            StepType:   "OrderService.Validate",
            Request:    marshalProto(t, &orderpb.ValidateRequest{OrderId: "order-123"}),
            Response:   marshalProto(t, &orderpb.ValidateResponse{Valid: true}),
        },
    }

    // Create durable context
    ctx := durable.NewContext(
        context.Background(),
        "test-invocation",
        journal,
        func(entry *durable.JournalEntry) error {
            // Capture journaled steps
            return nil
        },
    )

    // Execute workflow
    err := orderProcessingWorkflow(ctx)
    assert.NoError(t, err)
}
```

### Monitoring Workflow Execution

**Check processor logs:**
```bash
# View real-time logs
tail -f /path/to/processor.log

# Filter for specific invocation
grep "abc-123-def-456" /path/to/processor.log
```

**Query NATS JetStream:**
```bash
# View stream info
nats stream info COMMANDS

# View consumer info
nats consumer info COMMANDS processor-1

# View pending messages
nats consumer next COMMANDS processor-1 --count 10
```

**HTTP monitoring endpoints:**
```bash
# Check NATS server health
curl http://localhost:8222/healthz

# View JetStream stats
curl http://localhost:8222/jsz

# View stream details
curl http://localhost:8222/jsz?streams=true
```

### Creating New Services

1. Define your service in a `.proto` file:

```proto
syntax = "proto3";
package myservice;

service MyService {
  rpc DoSomething(Request) returns (Response);
}

message Request {
  string data = 1;
}

message Response {
  string result = 1;
}
```

2. Generate code:
```bash
make generate
```

3. Implement the service:

```go
type MyServiceImpl struct {
    pb.UnimplementedMyServiceServer
}

func (s *MyServiceImpl) DoSomething(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    // Your logic here
    return &pb.Response{Result: "done"}, nil
}
```

4. Register with gRPC server and update port mapping in `pkg/durable/context.go`

## How Durable Execution Works

### Deterministic Replay

When a workflow executes:

1. **First Execution**: Each step (gRPC call) is executed and journaled
2. **On Failure**: The workflow is retried from the beginning
3. **Replay**: Previously executed steps are loaded from the journal (no re-execution)
4. **Resume**: New steps are executed starting from where it failed
5. **Completion**: Final state is persisted

### Example

```
Execution 1:
  Step 1: Call ServiceA ✓ (journaled)
  Step 2: Call ServiceB ✗ (failed)

Execution 2 (replay):
  Step 1: Load from journal ✓ (no re-execution)
  Step 2: Call ServiceB ✓ (journaled)
  Step 3: Call ServiceC ✓ (journaled)
  Complete ✓
```

### State Persistence

Execution state is stored in NATS KV:

```go
ExecutionState {
  InvocationID: "uuid",
  Handler: "workflow_name",
  Journal: [
    {StepNumber: 0, StepType: "ServiceA.Method", Response: "..."},
    {StepNumber: 1, StepType: "ServiceB.Method", Response: "..."},
  ],
  Status: "running" | "completed" | "failed"
}
```

## Monitoring

### NATS Monitoring

Access the NATS monitoring endpoints:
- http://localhost:8222/varz - Server information
- http://localhost:8222/jsz - JetStream information
- http://localhost:8222/healthz - Health check

### Viewing Streams and KV

Using NATS CLI:

```bash
# Install NATS CLI
go install github.com/nats-io/natscli/nats@latest

# View streams
nats stream list

# View stream info
nats stream info COMMANDS

# View KV buckets
nats kv list

# View KV bucket entries
nats kv ls EXECUTION_STATE

# Get specific execution state
nats kv get EXECUTION_STATE <invocation-id>
```

## Development

### Build

```bash
make build
```

### Clean

```bash
make clean
```

### Run tests

```bash
make test
```

### Stop NATS cluster

```bash
make docker-down
```

## Configuration

### Environment Variables

**Processor:**
- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)
- `SUBMIT_TEST_COMMAND`: Submit a test command on startup (default: `false`)

**HelloService:**
- `GRPC_PORT`: gRPC server port (default: `9090`)

### Command-Line Flags

**Processor:**
- `--max-concurrent <int>`: Maximum number of concurrent workflow executions (MaxAckPending) (default: `100`)
- `--ack-wait <seconds>`: Timeout in seconds for workflow completion before redelivery (default: `300` / 5 minutes)

**Usage examples:**
```bash
# Low concurrency for resource-constrained environments
./bin/processor --max-concurrent 10 --ack-wait 120

# High concurrency for high-throughput systems
./bin/processor --max-concurrent 500 --ack-wait 600

# Balanced settings for production
./bin/processor --max-concurrent 100 --ack-wait 300
```

## Advanced Topics

### Partitioning

Commands are partitioned using the `partition_key` field, enabling parallel processing:

```go
cmd := &durable.Command{
    PartitionKey: "user-123",  // All commands for user-123 go to same partition
    // ...
}
```

### Exactly-Once Semantics

- Each `InvocationId` is unique (use UUID)
- JetStream deduplication window prevents duplicate commands
- State updates are atomic in KV
- Journal entries are immutable

### Failure Handling

- **Network Failures**: Automatic reconnection with exponential backoff
- **Service Failures**: Journaled as errors, workflow can handle or retry
- **Process Crashes**: Execution resumes from last persisted state
- **Partial Failures**: Only failed steps are re-executed

### Workflow Lifecycle Control

The system provides comprehensive workflow lifecycle management with pause, resume, and cancel capabilities.

#### Pause & Resume

Temporarily pause a workflow and resume it later from where it left off (same invocation ID):

```go
// Pause a running workflow
err := client.Pause(ctx, invocationID)

// Resume a paused workflow (continues from journal)
err := client.Resume(ctx, invocationID)
```

#### Cancel

Permanently stop a workflow execution:

```go
// Cancel a running or paused workflow
err := client.Cancel(ctx, invocationID)
```

#### Failure Detection & Retry

Identify failed workflows and retry them with new invocation IDs:

```go
// Get workflow result
result, err := client.GetResult(ctx, invocationID)

// Check if workflow failed
if result.Status == "failed" {
    // Retry with a new invocation ID
    newID, err := client.InvokeAsync(ctx, workflowName, client.WithArgs(input))
}
```

#### Comparison: Pause vs Cancel+Retry

| Feature | Pause/Resume | Cancel + Retry |
|---------|--------------|----------------|
| **Invocation ID** | Same ID continues | New ID created |
| **Journal** | Resumes from journal | Fresh start (or replay for idempotency) |
| **Use Case** | Temporary hold, waiting for resources/approval | Permanent cancellation, retry with modified input |
| **When to use** | Rate limiting, resource contention | Transient failures, input correction |

See [examples/README.md](examples/README.md) for detailed examples and demonstrations.

## Troubleshooting

### NATS not starting

```bash
# Check Docker logs
docker-compose logs nats-1

# Verify ports are available
lsof -i :4222
```

### Proto generation fails

```bash
# Verify protoc is installed
protoc --version

# Reinstall tools
make install-tools
```

### Processor can't connect to NATS

```bash
# Check NATS is running
docker ps | grep nats

# Check NATS health
curl http://localhost:8222/healthz

# Restart NATS
make docker-down && make docker-up
```

## Contributing

Contributions are welcome! Please follow these guidelines:

1. Write tests for new features
2. Follow Go best practices
3. Update documentation
4. Run `make test` before submitting

## License

MIT License

## References

- [NATS JetStream Documentation](https://docs.nats.io/nats-concepts/jetstream)
- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- [Protocol Buffers Guide](https://protobuf.dev/)
- [Durable Execution Patterns](https://temporal.io/durable-execution)
