# Simple Client SDK

A high-level SDK that abstracts away all the complexity of durable execution.

## The Problem

The original client (`cmd/client/main.go`) is **160 lines** of boilerplate:

```go
// 1. Connect to NATS
jsClient, err := jetstream.NewClient(natsURL)
if err != nil {
    log.Fatalf("Failed to connect to NATS: %v", err)
}
defer jsClient.Close()

// 2. Create processor
processor, err := execution.NewProcessor(jsClient)
if err != nil {
    log.Fatalf("Failed to create processor: %v", err)
}

// 3. Build command
cmd := &durable.Command{
    InvocationId: uuid.New().String(),
    Handler:      handlerName,
    Service:      "HelloService",
    Args:         []byte{},
    PartitionKey: partitionKey,
    Sequence:     1,
}

// 4. Submit command
if err := processor.SubmitCommand(ctx, cmd); err != nil {
    log.Fatalf("Failed to submit command: %v", err)
}

// 5. Poll for completion (50+ lines)
kv, err := jsClient.GetStateKV(ctx)
deadline := time.Now().Add(timeout)
ticker := time.NewTicker(500 * time.Millisecond)
defer ticker.Stop()

for {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-ticker.C:
        if time.Now().After(deadline) {
            return nil, fmt.Errorf("timeout")
        }
        entry, err := kv.Get(ctx, invocationID)
        // ... more polling logic
    }
}

// 6. Unmarshal state
var state durable.ExecutionState
if err := proto.Unmarshal(entry.Value(), &state); err != nil {
    return nil, err
}

// 7. Extract response
var helloResp hellopb.HelloResponse
if err := proto.Unmarshal(entry.Response, &helloResp); err != nil {
    return nil, err
}

fmt.Println(helloResp.Message)
```

## The Solution

With the new SDK (`pkg/client`), it's just **5-10 lines**:

```go
package main

import (
    "context"
    "fmt"

    hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
    "github.com/sanjaymijar/my-durable-execution/pkg/client"
)

func main() {
    c, _ := client.NewClient("nats://127.0.0.1:4322")
    defer c.Close()

    result, _ := c.Invoke(context.Background(), "hello_workflow")

    var resp hellopb.HelloResponse
    result.GetFirstResponse(&resp)

    fmt.Println(resp.Message)
}
```

**Output:**
```
Hello, World! Welcome to Durable Execution on NATS JetStream.
```

## What's Hidden

The SDK handles all of this automatically:

✅ NATS connection management
✅ UUID generation
✅ Command construction
✅ Submission to JetStream
✅ Polling the KV bucket
✅ Timeout handling
✅ State unmarshaling
✅ Response extraction
✅ Error handling
✅ Resource cleanup

## API

### Synchronous Invocation

Wait for the workflow to complete:

```go
result, err := c.Invoke(ctx, "hello_workflow")
```

### Asynchronous Invocation

Fire and forget:

```go
invocationID, err := c.InvokeAsync(ctx, "hello_workflow")

// Do other work...

// Check result later
result, err := c.GetResult(ctx, invocationID)
```

### With Options

```go
result, err := c.Invoke(
    ctx,
    "hello_workflow",
    client.WithPartitionKey("user-123"),
    client.WithService("HelloService"),
)
```

### Custom Timeout

```go
c, err := client.NewClientWithTimeout(
    "nats://127.0.0.1:4322",
    5*time.Second, // Custom timeout
)
```

### Extract Response

```go
// Get first step response
var resp hellopb.HelloResponse
err := result.GetFirstResponse(&resp)

// Or get specific step
err := result.GetResponse(0, &resp)
```

## Running Examples

```bash
# Make sure services are running
make docker-up       # Terminal 1
make run-service     # Terminal 2
make run-processor   # Terminal 3

# Run simple example
make run-simple

# Run async example
make run-async
```

Or with go run:

```bash
go run examples/simple/main.go
go run examples/async/main.go
```

## Comparison

| Feature | Original Client | Simple SDK |
|---------|----------------|------------|
| Lines of code | 160 | 5-10 |
| NATS setup | Manual | Automatic |
| Polling loop | Manual | Automatic |
| Timeout handling | Manual | Automatic |
| Response extraction | Manual | One method call |
| Error handling | Everywhere | Centralized |
| Learning curve | High | Low |

## Use Cases

### Quick Scripts

```go
func main() {
    c, _ := client.NewClient("nats://127.0.0.1:4322")
    defer c.Close()

    result, _ := c.Invoke(context.Background(), "process_order")
    fmt.Println("Order processed!")
}
```

### Background Jobs

```go
func submitJobs() {
    c, _ := client.NewClient("nats://127.0.0.1:4322")
    defer c.Close()

    for _, order := range orders {
        c.InvokeAsync(ctx, "process_order",
            client.WithPartitionKey(order.UserID))
    }
}
```

### Waiting for Results

```go
func processWithResult() {
    c, _ := client.NewClient("nats://127.0.0.1:4322")
    defer c.Close()

    result, _ := c.Invoke(ctx, "calculate_total")

    var resp TotalResponse
    result.GetFirstResponse(&resp)

    fmt.Printf("Total: $%.2f\n", resp.Amount)
}
```

## Architecture

```
┌─────────────────┐
│  Your Code      │  5 lines
│  (examples/)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Client SDK     │  Hides complexity
│  (pkg/client)   │
└────────┬────────┘
         │
         ├──────────────────┐
         │                  │
         ▼                  ▼
┌─────────────────┐  ┌──────────────┐
│  JetStream      │  │  Execution   │
│  (pkg/jetstream)│  │  (pkg/exec)  │
└─────────────────┘  └──────────────┘
```

## Benefits

1. **Simplicity** - Minimal code for common use cases
2. **Safety** - Centralized error handling and resource cleanup
3. **Flexibility** - Options for sync/async invocation
4. **Maintainability** - SDK handles protocol changes
5. **Productivity** - Focus on business logic, not plumbing

## Next Steps

1. Check out `examples/simple/main.go` for the minimal example
2. See `examples/async/main.go` for fire-and-forget pattern
3. Read `examples/README.md` for more patterns

---

**From 160 lines to 5 lines. That's a 97% reduction in code!** 🎉
