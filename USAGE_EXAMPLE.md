# Usage Examples

This document provides step-by-step examples for using the Durable Execution Engine.

## Quick Start: Your First Workflow

### 1. Start the System

Open three terminals and run these commands:

**Terminal 1 - NATS Cluster:**
```bash
cd my-durable-execution
make docker-up
```

Wait for NATS to start (about 5 seconds). You should see:
```
✓ Container nats-1    Started
✓ Container nats-2    Started
✓ Container nats-3    Started
NATS cluster is ready
```

**Terminal 2 - HelloService:**
```bash
make run-service
```

Output:
```
Running HelloService...
2024/11/10 22:00:00 Starting HelloService gRPC server...
2024/11/10 22:00:00 HelloService listening on :9090
```

**Terminal 3 - Processor:**
```bash
make run-processor
```

Output:
```
Running processor...
2024/11/10 22:00:00 Starting Durable Execution Processor...
2024/11/10 22:00:00 Setting up JetStream streams...
2024/11/10 22:00:00 Setting up KV buckets...
2024/11/10 22:00:01 Processor started, consuming from processor-1
```

### 2. Submit a Workflow

Open a fourth terminal and submit a command:

```bash
make run-client
```

Output:
```
Running client...
2024/11/10 22:01:00 Starting Durable Execution Client...
2024/11/10 22:01:00 Submitting workflow:
2024/11/10 22:01:00   Invocation ID: a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e
2024/11/10 22:01:00   Handler: hello_workflow
2024/11/10 22:01:00   Partition Key: default
2024/11/10 22:01:00 ✓ Command submitted successfully!
2024/11/10 22:01:00 Track execution with: nats kv get EXECUTION_STATE a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e
```

### 3. Watch the Execution

In Terminal 3 (Processor), you'll see:
```
2024/11/10 22:01:00 Processing command: invocation_id=a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e handler=hello_workflow service=HelloService
2024/11/10 22:01:00 Executing hello workflow: invocation_id=a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e
2024/11/10 22:01:00 Hello workflow completed: message=Hello, World! Welcome to Durable Execution on NATS JetStream.
2024/11/10 22:01:00 Execution completed successfully: a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e
```

In Terminal 2 (HelloService), you'll see:
```
2024/11/10 22:01:00 Received SayHello request: name=World
```

Congratulations! You just executed your first durable workflow! 🎉

## Example 2: Submitting Multiple Workflows

Submit multiple workflows with different partition keys:

```bash
# Submit workflow for user-123
PARTITION_KEY=user-123 make run-client

# Submit workflow for user-456
PARTITION_KEY=user-456 make run-client

# Submit workflow for user-789
PARTITION_KEY=user-789 make run-client
```

Each workflow will be processed independently by the processor. Workflows with the same partition key are guaranteed to be processed in order.

## Example 3: Testing Failure Recovery

Let's test how durable execution handles failures.

### Step 1: Start the system

```bash
# Terminal 1
make docker-up

# Terminal 2
make run-service

# Terminal 3
make run-processor
```

### Step 2: Submit a workflow

```bash
# Terminal 4
make run-client
```

Note the invocation ID from the output.

### Step 3: Kill the processor mid-execution

In Terminal 3, press `Ctrl+C` to stop the processor.

### Step 4: Restart the processor

```bash
make run-processor
```

The workflow will automatically resume from where it left off! Check the logs - you'll see it replays completed steps from the journal and only re-executes new steps.

## Example 4: Inspecting Execution State

Install NATS CLI:
```bash
go install github.com/nats-io/natscli/nats@latest
```

### View all execution states

```bash
nats kv ls EXECUTION_STATE
```

Output:
```
EXECUTION_STATE> a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e
EXECUTION_STATE> b4e6d9a0-8c2f-5d3b-0e4g-9f8b7c6d5e4f
EXECUTION_STATE> c5f7e0b1-9d3g-6e4c-1f5h-0g9c8d7e6f5g
```

### Get specific execution state

```bash
nats kv get EXECUTION_STATE a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e
```

Output (binary protobuf, but shows metadata):
```
EXECUTION_STATE/a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e created @ 10 Nov 24 22:01 UTC
```

### Watch for state changes in real-time

```bash
nats kv watch EXECUTION_STATE
```

This will show all state updates as they happen!

## Example 5: Custom Workflow with Environment Variables

You can customize the client behavior with environment variables:

```bash
# Submit to a specific handler
HANDLER=custom_workflow make run-client

# Use a specific partition key
PARTITION_KEY=tenant-abc make run-client

# Connect to a different NATS server
NATS_URL=nats://nats.example.com:4222 make run-client

# Combine multiple
HANDLER=order_processing PARTITION_KEY=order-12345 make run-client
```

## Example 6: Monitoring with NATS CLI

### View stream information

```bash
nats stream info COMMANDS
```

Output:
```
Information for Stream COMMANDS created 2024-11-10 22:00:00

Configuration:

             Subjects: commands.>
             Replicas: 3
              Storage: File

Statistics:

             Messages: 42
                Bytes: 12.3 KB
             FirstSeq: 1
              LastSeq: 42
```

### View consumer information

```bash
nats consumer info COMMANDS processor-1
```

Output:
```
Information for Consumer COMMANDS > processor-1

Configuration:

        Durable Name: processor-1
           Pull Mode: true
         Ack Policy: explicit
          Ack Wait: 30.00s
     Max Deliver: 3

State:

  Last Delivered Message: Consumer sequence: 42 Stream sequence: 42
    Acknowledgment floor: Consumer sequence: 42 Stream sequence: 42
        Outstanding Acks: 0 out of maximum 1000
    Redelivered Messages: 0
```

### Check NATS health

```bash
curl http://localhost:8222/healthz
```

Output:
```
ok
```

### View JetStream statistics

```bash
curl http://localhost:8222/jsz | jq
```

Output (formatted JSON):
```json
{
  "now": "2024-11-10T22:00:00Z",
  "config": {
    "max_memory": 1073741824,
    "max_storage": 10737418240
  },
  "memory": 1024000,
  "storage": 12300,
  "streams": 1,
  "consumers": 1
}
```

## Example 7: Programmatic State Inspection

Create a tool to inspect execution state:

**`cmd/inspect/main.go`:**
```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/sanjaymijar/my-durable-execution/pb/durable"
    "github.com/sanjaymijar/my-durable-execution/pkg/jetstream"
    "google.golang.org/protobuf/proto"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: inspect <invocation-id>")
    }

    invocationID := os.Args[1]

    // Connect to NATS
    jsClient, err := jetstream.NewClient("nats://localhost:4222")
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer jsClient.Close()

    ctx := context.Background()

    // Get KV bucket
    kv, err := jsClient.GetStateKV(ctx)
    if err != nil {
        log.Fatalf("Failed to get KV: %v", err)
    }

    // Get execution state
    entry, err := kv.Get(ctx, invocationID)
    if err != nil {
        log.Fatalf("Failed to get state: %v", err)
    }

    var state durable.ExecutionState
    if err := proto.Unmarshal(entry.Value(), &state); err != nil {
        log.Fatalf("Failed to unmarshal: %v", err)
    }

    // Print state
    fmt.Printf("Execution State:\n")
    fmt.Printf("  Invocation ID: %s\n", state.InvocationId)
    fmt.Printf("  Handler: %s\n", state.Handler)
    fmt.Printf("  Status: %s\n", state.Status)
    fmt.Printf("  Journal Entries: %d\n", len(state.Journal))
    fmt.Printf("\nJournal:\n")
    for i, entry := range state.Journal {
        fmt.Printf("  Step %d: %s\n", entry.StepNumber, entry.StepType)
        if entry.Error != "" {
            fmt.Printf("    Error: %s\n", entry.Error)
        }
    }
}
```

Build and run:
```bash
go build -o bin/inspect ./cmd/inspect
./bin/inspect a3f5c8d9-7b1e-4c2a-9d3f-8e7a6b5c4d3e
```

## Cleanup

When you're done:

```bash
# Stop services (Ctrl+C in each terminal)

# Stop and remove NATS cluster
make docker-down

# Clean built binaries
make clean
```

## Next Steps

1. **Create Custom Workflows**: Edit `cmd/processor/main.go` to add new handlers
2. **Add More Services**: Define new proto files and implement gRPC services
3. **Scale Horizontally**: Run multiple processor instances with different consumer names
4. **Production Deployment**: Deploy to Kubernetes with NATS operators

Happy building with Durable Execution! 🚀
