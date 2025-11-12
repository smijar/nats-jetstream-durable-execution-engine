# Examples

Simple examples showing how to use the Durable Execution Client SDK.

## Super Simple (5 lines!)

```go
c, _ := client.NewClient("nats://127.0.0.1:4322")
defer c.Close()

result, _ := c.Invoke(context.Background(), "hello_workflow")
var resp hellopb.HelloResponse
result.GetFirstResponse(&resp)

fmt.Println(resp.Message)
// Output: Hello, World! Welcome to Durable Execution on NATS JetStream.
```

**Run it:**
```bash
go run examples/simple/main.go
```

## With Proper Error Handling

```go
c, err := client.NewClient("nats://127.0.0.1:4322")
if err != nil {
    log.Fatal(err)
}
defer c.Close()

result, err := c.Invoke(context.Background(), "hello_workflow")
if err != nil {
    log.Fatal(err)
}

var resp hellopb.HelloResponse
if err := result.GetFirstResponse(&resp); err != nil {
    log.Fatal(err)
}

fmt.Println("Response:", resp.Message)
```

## Async Invocation (Fire and Forget)

```go
c, _ := client.NewClient("nats://127.0.0.1:4322")
defer c.Close()

// Submit without waiting
invocationID, _ := c.InvokeAsync(context.Background(), "hello_workflow")
fmt.Printf("Submitted: %s\n", invocationID)

// Do other work...
time.Sleep(2 * time.Second)

// Check result later
result, _ := c.GetResult(context.Background(), invocationID)
var resp hellopb.HelloResponse
result.GetFirstResponse(&resp)
fmt.Println(resp.Message)
```

**Run it:**
```bash
go run examples/async/main.go
```

## With Options

```go
result, err := c.Invoke(
    context.Background(),
    "hello_workflow",
    client.WithPartitionKey("user-123"),
    client.WithService("HelloService"),
)
```

## Custom Timeout

```go
// 5 second timeout
c, err := client.NewClientWithTimeout("nats://127.0.0.1:4322", 5*time.Second)
```

## Before Running Examples

Make sure the services are running:

```bash
# Terminal 1: Start NATS
make docker-up

# Terminal 2: Start HelloService
make run-service

# Terminal 3: Start Processor
make run-processor

# Terminal 4: Run examples
go run examples/simple/main.go
```

## Comparison

### Before (160 lines in cmd/client/main.go)
- Connect to NATS manually
- Create processor
- Build command with UUID
- Submit command
- Poll KV bucket in a loop
- Check for timeout
- Unmarshal state
- Extract and decode responses
- Handle errors at every step

### After (5-10 lines)
```go
c, _ := client.NewClient("nats://127.0.0.1:4322")
result, _ := c.Invoke(ctx, "hello_workflow")
var resp hellopb.HelloResponse
result.GetFirstResponse(&resp)
fmt.Println(resp.Message)
```

**All complexity hidden in the SDK!** ✨

---

## Advanced Examples

### Type-Safe Workflows (`examples/typed/`)

Use generics for compile-time type safety:

```go
// Define a type-safe workflow
var HelloWorkflow = durable.NewWorkflow("hello_workflow",
    func(ctx *durable.Context, name string) (string, error) {
        req := &hellopb.HelloRequest{Name: name}
        resp := &hellopb.HelloResponse{}

        if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
            return "", err
        }

        return resp.Message, nil
    })

// Invoke with type safety - compiler checks input/output types
result, err := client.InvokeWorkflow[string, string](c, ctx, HelloWorkflow, "TypeSafe World")
fmt.Println(result) // string type, no unmarshaling needed!
```

**Benefits:**
- Compile-time type checking
- No manual marshaling/unmarshaling
- Refactoring-friendly
- IDE autocomplete support

**Run it:**
```bash
make run-typed
```

---

### Local Development Mode (`examples/local/`)

Run workflows locally without NATS for faster development:

```go
c, _ := client.NewClient("nats://127.0.0.1:4322")

// Register workflow handler locally
c.Register(HelloWorkflow)
go c.ServeHandlers(ctx)

// Now invocations run IN-PROCESS, not via NATS!
result, _ := client.InvokeWorkflow[string, string](c, ctx, HelloWorkflow, "Local World")
fmt.Println("✓ Executed locally!")
fmt.Println("Result:", result)
```

**Benefits:**
- No network latency
- Easier debugging with breakpoints
- Faster iteration cycles
- Same code runs locally or distributed

**Run it:**
```bash
make run-local
```

---

### Multi-Service Invocations (`examples/multi-service/`)

**Call the same service multiple times in one workflow** - each call is journaled separately for durability:

```go
var MultiServiceWorkflow = durable.NewWorkflow("multi_service_demo",
    func(ctx *durable.Context, topic string) (string, error) {
        // Call 1: English greeting
        req1 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (English)", topic)}
        resp1 := &hellopb.HelloResponse{}
        if err := ctx.DurableCall("HelloService", "SayHello", req1, resp1); err != nil {
            return "", err
        }

        // Call 2: Spanish greeting
        req2 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (Spanish)", topic)}
        resp2 := &hellopb.HelloResponse{}
        if err := ctx.DurableCall("HelloService", "SayHello", req2, resp2); err != nil {
            return "", err
        }

        // Call 3: French greeting
        req3 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (French)", topic)}
        resp3 := &hellopb.HelloResponse{}
        if err := ctx.DurableCall("HelloService", "SayHello", req3, resp3); err != nil {
            return "", err
        }

        // Call 4: Summary
        req4 := &hellopb.HelloRequest{Name: "Summary"}
        resp4 := &hellopb.HelloResponse{}
        if err := ctx.DurableCall("HelloService", "SayHello", req4, resp4); err != nil {
            return "", err
        }

        // Combine results
        return fmt.Sprintf(
            "Results:\n1️⃣  %s\n2️⃣  %s\n3️⃣  %s\n📊 %s",
            resp1.Message, resp2.Message, resp3.Message, resp4.Message,
        ), nil
    })
```

**Key Features:**
- Each `ctx.DurableCall()` is journaled separately
- If workflow fails after Call 2, replay skips Calls 1-2
- Exactly-once semantics per service call
- Works with any number of service calls
- Can call different services or same service multiple times

**Run it:**
```bash
make run-multi-service
```

**Output:**
```
Multi-Service Workflow Results:
1️⃣  Hello, Durable Execution (English)! Welcome to Durable Execution on NATS JetStream.
2️⃣  Hello, Durable Execution (Spanish)! Welcome to Durable Execution on NATS JetStream.
3️⃣  Hello, Durable Execution (French)! Welcome to Durable Execution on NATS JetStream.
📊 Hello, Summary! Welcome to Durable Execution on NATS JetStream.
Total service calls: 4
```

---

### Concurrent Workflow Invocations (`examples/concurrent-workflows/`)

**Invoke the same workflow multiple times concurrently** with configurable count:

```go
// Define a 3-step workflow
var AsyncWorkflow = durable.NewWorkflow("async_workflow",
    func(ctx *durable.Context, userID string) (string, error) {
        // Step 1: Synchronous greeting
        req1 := &hellopb.HelloRequest{Name: fmt.Sprintf("User:%s", userID)}
        resp1 := &hellopb.HelloResponse{}
        ctx.DurableCall("HelloService", "SayHello", req1, resp1)

        // Step 2: Async-style notification
        req2 := &hellopb.HelloRequest{Name: fmt.Sprintf("Async:%s", userID)}
        resp2 := &hellopb.HelloResponse{}
        ctx.DurableCall("HelloService", "SayHello", req2, resp2)

        // Step 3: Final confirmation
        req3 := &hellopb.HelloRequest{Name: fmt.Sprintf("Confirm:%s", userID)}
        resp3 := &hellopb.HelloResponse{}
        ctx.DurableCall("HelloService", "SayHello", req3, resp3)

        return fmt.Sprintf("Completed: %s, %s, %s",
            resp1.Message, resp2.Message, resp3.Message), nil
    })

// Invoke N times concurrently
for i := 0; i < N; i++ {
    go func(userID string) {
        result, _ := client.InvokeWorkflow[string, string](c, ctx, AsyncWorkflow, userID)
        fmt.Println(result)
    }(fmt.Sprintf("user-%03d", i))
}
```

**Key Features:**
- Configurable number of concurrent invocations via `--count` flag (default: 10)
- Configurable timeout via `--timeout` flag in seconds (default: 300s / 5min)
- Each invocation gets a unique user ID
- All workflows execute in parallel via NATS/processor
- Summary shows total time and average time per workflow
- Demonstrates system scalability and concurrency

**Run it:**
```bash
# Default: 10 concurrent invocations (5 minute timeout)
make run-concurrent

# Custom count
./bin/concurrent-workflows --count 5
./bin/concurrent-workflows --count 20
./bin/concurrent-workflows --count 100

# Large batches with custom timeout (in seconds)
./bin/concurrent-workflows --count 10000 --timeout 600    # 10 min timeout
./bin/concurrent-workflows --count 50000 --timeout 1800   # 30 min timeout
```

**Output Example:**
```
Concurrent Workflow Invocations (N=10)
========================================
🚀 Submitting 10 workflow invocations...

📤 [Invoke 1] Submitting workflow for user-001
📤 [Invoke 2] Submitting workflow for user-002
...
✅ [Invoke 1] Completed for user-001
✅ [Invoke 2] Completed for user-002

========================================
Results Summary
========================================
AsyncWorkflow[user-001] completed:
  Step1: Hello, User:user-001! Welcome...
  Step2: Hello, Async:user-001! Welcome...
  Step3: Hello, Confirm:user-001! Welcome...

Total Invocations: 10
✅ Success: 10
❌ Failed: 0
⏱️  Total Time: 505ms
📊 Avg Time per Workflow: 50ms
```

**Use Cases:**
- Load testing your durable workflows
- Bulk processing (e.g., 100 user notifications)
- Demonstrating system scalability
- Testing concurrent execution guarantees

**Performance Benchmarks:**
| Invocations | Total Time | Avg per Workflow | Throughput |
|-------------|------------|------------------|------------|
| 5           | ~506ms     | ~101ms           | ~10 workflows/sec |
| 10          | ~505ms     | ~50ms            | ~20 workflows/sec |
| 20          | ~1s        | ~50ms            | ~20 workflows/sec |

*Note: Each workflow makes 3 sequential gRPC calls. Performance depends on network latency and processor capacity.*

**How It Works:**
1. Client submits N workflow commands to NATS concurrently
2. Processor picks up commands and executes workflows
3. Each workflow's 3 steps are journaled separately for durability
4. If a workflow fails mid-execution, replay resumes from the last completed step
5. Client polls for completion and aggregates results

---

### Failure Detection & Retry (`examples/retry-failures/`)

**Detect failed workflows and selectively retry them:**

```go
// Phase 1: Submit workflows (some will fail)
inputs := []string{"task-success-1", "task-flaky-1", "task-success-2", "task-flaky-2"}
invocations := []workflowInvocation{}

for _, input := range inputs {
    inputBytes, _ := json.Marshal(input)
    invocationID, _ := c.InvokeAsync(ctx, "flaky_workflow", client.WithArgs(inputBytes))
    invocations = append(invocations, workflowInvocation{invocationID, input})
}

// Phase 2: Identify failures
failedWorkflows := []workflowInvocation{}
for _, inv := range invocations {
    result, _ := c.GetResult(ctx, inv.invocationID)
    if result.Status == "failed" {
        failedWorkflows = append(failedWorkflows, inv)  // Collect failures
    }
}

// Phase 3: Retry only failed workflows
for _, failed := range failedWorkflows {
    retryInput := failed.input + "-retry"
    inputBytes, _ := json.Marshal(retryInput)
    newID, _ := c.InvokeAsync(ctx, "flaky_workflow", client.WithArgs(inputBytes))
    // Each retry gets a NEW invocation ID
}
```

**Key Features:**
- Detect failures by checking `result.Status == "failed"`
- Collect failed invocation IDs in an array
- Retry creates a **new invocation** with a **new invocation ID**
- Can modify input for retry (e.g., add retry token, backoff delay)
- 100% success rate after retry in the demo

**Run it:**
```bash
make run-retry
```

**Output Example:**
```
📊 PHASE 2: Checking workflow status...
  Workflow: task-success-1
    Status: completed ✅ SUCCESS

  Workflow: task-flaky-1
    Status: failed ❌ FAILED

🔄 PHASE 4: Retrying failed workflows...
  Retrying: task-flaky-1 → task-flaky-1-retry
    ✓ Resubmitted (invocation_id: f49bf85e...)

✅ PHASE 5: Verifying retry results...
  Retry: task-flaky-1-retry
    Status: completed ✅ RETRY SUCCEEDED!

Overall Success Rate: 4/4 (100.0%)
```

**Use Cases:**
- Handling transient failures (network timeouts, service unavailable)
- Retry with exponential backoff
- Bulk job processing with error recovery
- Idempotent workflow retries

**Important Notes:**
- Each retry is a **new workflow invocation** with a **new ID**
- Original failed workflow state remains in KV store
- Retry can use modified input (e.g., with retry count)
- For continuing paused workflows with same ID, use **Pause/Resume** instead

---

### Pause/Resume & Cancel (`examples/pause-cancel/`)

**Pause workflows temporarily or cancel them permanently:**

```go
// Submit a workflow
inputBytes, _ := json.Marshal("my-task")
invocationID, _ := c.InvokeAsync(ctx, "slow_workflow", client.WithArgs(inputBytes))

// Let it run for a bit...
time.Sleep(1 * time.Second)

// PAUSE the workflow (same invocation ID continues)
c.Pause(ctx, invocationID)

// Check status
result, _ := c.GetResult(ctx, invocationID)
fmt.Printf("Status: %s\n", result.Status)  // "paused"
fmt.Printf("Journal entries: %d\n", len(result.Journal))  // Steps completed so far

// RESUME from where it left off
c.Resume(ctx, invocationID)

// Or CANCEL permanently
c.Cancel(ctx, invocationID)
result, _ = c.GetResult(ctx, invocationID)
fmt.Printf("Status: %s\n", result.Status)  // "cancelled"
```

**Key Features:**
- **Pause**: Sets status to "paused" in KV store
- **Resume**: Sets status back to "pending", processor continues from journal
- **Cancel**: Sets status to "cancelled", processor stops execution
- **Same invocation ID** for pause/resume (continues where it left off)
- **Journal-based resume** - completed steps aren't re-executed

**Run it:**
```bash
make run-pause
```

**Output Example:**
```
📋 DEMO 1: Pause & Resume
✓ Submitted workflow (invocation_id: 51168e91...)

⏸️  PAUSING workflow...
✓ Workflow paused

Status check: paused
Journal entries: 3

▶️  RESUMING workflow...
✓ Workflow resumed

✅ Final Status: completed

📋 DEMO 2: Cancel Workflow
🛑 CANCELLING workflow...
✓ Workflow cancelled

✅ Final Status: cancelled
```

**Use Cases:**
- **Pause**: Rate limiting, waiting for external resources, manual approval gates
- **Resume**: Continue after resources available or approval received
- **Cancel**: User-requested cancellation, timeout, invalid state

**Pause vs Retry:**

| Feature | Pause/Resume | Cancel + Retry |
|---------|-------------|----------------|
| Invocation ID | **Same ID continues** | **New ID created** |
| Journal | Resumes from journal | Fresh start (or replay for idempotency) |
| Use Case | Temporary hold | Permanent cancellation, retry with new input |
| When to use | Waiting for resources, approval | Transient failures, input correction |

---

### Restate-Style API (`examples/restate-style/`)

Comprehensive example showing all patterns:

```go
// 1. Type-safe remote invocation
result1, _ := client.InvokeWorkflow[string, string](c, ctx, GreetingWorkflow, "Restate-Style")

// 2. Workflow references (invoke without local handler)
remoteRef := GreetingWorkflow.Ref()
result2, _ := client.InvokeWorkflow[string, string](c, ctx, remoteRef, "Remote")

// 3. Local development mode
c.Register(GreetingWorkflow)
go c.ServeHandlers(ctx)
result3, _ := client.InvokeWorkflow[string, string](c, ctx, GreetingWorkflow, "Local")

// 4. Workflow composition
result4, _ := client.InvokeWorkflow[[]string, []string](c, ctx, GreetManyWorkflow,
    []string{"Alice", "Bob", "Charlie"})
```

**Run it:**
```bash
make run-restate
```

---

### Workflow State Management (`examples/ticket-cart/`)

**Restate-style workflow-scoped state** - persist data across workflow steps:

```go
var AddTicketToCartWorkflow = durable.NewWorkflow("add_ticket_to_cart",
    func(ctx *durable.Context, input AddTicketToCartInput) (bool, error) {
        // Step 1: Reserve the ticket (durable RPC call)
        req := &ticketpb.ReserveRequest{TicketId: input.TicketID}
        resp := &ticketpb.ReserveResponse{}
        ctx.DurableCall("TicketService", "Reserve", req, resp)

        if resp.Success {
            // Step 2: Get current cart from workflow state
            var cart []string
            err := ctx.Get("cart", &cart)
            if err != nil {
                cart = []string{}  // Cart doesn't exist yet
            }

            // Add ticket to cart
            cart = append(cart, input.TicketID)

            // Save updated cart (automatically persisted)
            ctx.Set("cart", cart)

            // Step 3: Schedule expiry timer (15 minutes)
            expireInput := ExpireTicketInput{
                UserID:   input.UserID,
                TicketID: input.TicketID,
            }
            ctx.SendDelayed("expire_ticket", expireInput, 15*time.Minute, input.UserID)
        }

        return resp.Success, nil
    })
```

**Key Features:**
- `ctx.Get(key, &value)` - Retrieve workflow-scoped state
- `ctx.Set(key, value)` - Store workflow-scoped state
- State persists across failures and replays
- State stored in NATS KV, automatically marshaled to JSON
- Similar to Restate's `ctx.get()` / `ctx.set()`

**Run it:**
```bash
# Terminal 1: Start services
make docker-up
make run-service

# Terminal 2: Start TicketService
make run-ticket-service

# Terminal 3: Start processor
make run-processor

# Terminal 4: Run example
make run-ticket-cart
```

**Use Cases:**
- Shopping carts
- User sessions
- Workflow-scoped counters
- Multi-step form data
- Accumulating results

---

### Delayed Execution (`examples/delayed-execution/`)

**Schedule workflows to run in the future** - like Restate's `ctx.sendDelayed()`:

```go
var SchedulerWorkflow = durable.NewWorkflow("scheduler",
    func(ctx *durable.Context, input SchedulerInput) (string, error) {
        // Do some work first
        req := &hellopb.HelloRequest{Name: "Scheduler"}
        resp := &hellopb.HelloResponse{}
        ctx.DurableCall("HelloService", "SayHello", req, resp)

        // Schedule a delayed workflow invocation
        delayDuration := time.Duration(input.Delay) * time.Second
        greetingInput := GreetingInput{Name: input.Message}

        // This schedules the workflow to run after the delay
        ctx.SendDelayed("delayed_greeting", greetingInput, delayDuration, "partition-key")

        return "Scheduled greeting for future execution", nil
    })

// This workflow gets invoked after the delay
var DelayedGreetingWorkflow = durable.NewWorkflow("delayed_greeting",
    func(ctx *durable.Context, input GreetingInput) (string, error) {
        // This runs after the delay period
        req := &hellopb.HelloRequest{Name: input.Name}
        resp := &hellopb.HelloResponse{}
        ctx.DurableCall("HelloService", "SayHello", req, resp)

        return resp.Message, nil
    })
```

**Key Features:**
- `ctx.SendDelayed(handler, args, delay, partitionKey)` - Schedule future workflow
- Uses NATS headers to track scheduled time
- Processor automatically requeues until ready
- Survives processor restarts
- Similar to Restate's `ctx.sendDelayed()`

**Run it:**
```bash
make run-delayed
```

**Use Cases:**
- Timeout/expiry timers (expire cart items)
- Reminder notifications
- Scheduled maintenance tasks
- Rate limiting with delays
- Retry with exponential backoff

---

### Awakeables / External Callbacks (`examples/approval-workflow/`)

**Pause workflows waiting for external events** - like Restate's `ctx.awakeable()`:

```go
var ExpenseApprovalWorkflow = durable.NewWorkflow("expense_approval",
    func(ctx *durable.Context, request ExpenseRequest) (string, error) {
        // Step 1: Log the expense request
        req := &hellopb.HelloRequest{Name: fmt.Sprintf("Expense from %s", request.EmployeeID)}
        resp := &hellopb.HelloResponse{}
        ctx.DurableCall("HelloService", "SayHello", req, resp)

        // Step 2: Create awakeable and wait for manager approval
        awakeableID := fmt.Sprintf("approval-%s", uuid.New().String())

        log.Printf("⏳ Waiting for manager approval (ID: %s)", awakeableID)
        log.Printf("Manager can approve via HTTP:")
        log.Printf("  curl -X POST http://localhost:8080/api/awakeables/%s/resolve \\", awakeableID)
        log.Printf("    -H 'Content-Type: application/json' \\")
        log.Printf("    -d '{\"result\": \"{\\\"approved\\\": true, \\\"comments\\\": \\\"OK!\\\"}\"}'")

        // Workflow SUSPENDS here until external system resolves the awakeable
        resultBytes, err := ctx.Awakeable(awakeableID)
        if err != nil {
            return "Rejected", err
        }

        // Step 3: Parse approval result and continue
        var approval ApprovalResult
        json.Unmarshal(resultBytes, &approval)

        if approval.Approved {
            // Notify employee of approval
            notifyReq := &hellopb.HelloRequest{Name: fmt.Sprintf("Approved: %s", request.EmployeeID)}
            notifyResp := &hellopb.HelloResponse{}
            ctx.DurableCall("HelloService", "SayHello", notifyReq, notifyResp)

            return fmt.Sprintf("✅ Approved: $%.2f for %s", request.Amount, request.EmployeeID), nil
        }

        return fmt.Sprintf("❌ Rejected: $%.2f", request.Amount), nil
    })
```

**Resolving Awakeables (Manager's Action):**

```bash
# Approve the expense
curl -X POST http://localhost:8080/api/awakeables/approval-abc123/resolve \
  -H 'Content-Type: application/json' \
  -d '{"result": "{\"approved\": true, \"comments\": \"Looks good!\"}"}'

# Or reject it
curl -X POST http://localhost:8080/api/awakeables/approval-abc123/reject \
  -H 'Content-Type: application/json' \
  -d '{"error": "Amount too high"}'
```

**Key Features:**
- `ctx.Awakeable(id)` - Create durable promise, wait for external resolution
- Workflow suspends execution (processor requeues message)
- HTTP API for resolving awakeables (`/api/awakeables/{id}/resolve` or `/reject`)
- Survives processor crashes while waiting
- Result journaled for replay
- Similar to Restate's `ctx.awakeable()`

**Run it:**
```bash
# Terminal 1-3: Start services
make docker-up
make run-service
make run-processor  # Must include HTTP API on port 8080

# Terminal 4: Run example
make run-approval

# Terminal 5: Approve/reject (check processor logs for exact command)
curl -X POST http://localhost:8080/api/awakeables/approval-<ID>/resolve \
  -H 'Content-Type: application/json' \
  -d '{"result": "{\"approved\": true, \"comments\": \"Approved!\"}"}'
```

**Use Cases:**
- **Human approvals** - Manager approves expense, PR review
- **Payment webhooks** - Stripe, PayPal notify payment completion
- **External API callbacks** - 3rd party service completes long job
- **Email confirmations** - User clicks link to proceed
- **IoT/sensor data** - Wait for device to report
- **Manual intervention** - Admin resolves issue

**How It Works:**
1. Workflow creates awakeable with unique ID
2. Workflow suspends (returns `AwakeableSuspendError`)
3. Processor requeues message instead of failing
4. External system calls HTTP API to resolve
5. Processor updates awakeable state in KV
6. Next message redelivery resumes workflow with result

---

### Query API (`examples/query-api/`)

**Query workflow status, journal, and state via HTTP API:**

```bash
# Get all workflows
curl http://localhost:8080/api/workflows

# Get specific workflow
curl http://localhost:8080/api/workflows/{invocation_id}

# Get workflow stats
curl http://localhost:8080/api/workflows/stats

# Filter by status
curl "http://localhost:8080/api/workflows?status=completed"
curl "http://localhost:8080/api/workflows?status=failed"

# Filter by handler
curl "http://localhost:8080/api/workflows?handler=expense_approval"

# Limit results
curl "http://localhost:8080/api/workflows?limit=10"
```

**Response Example:**
```json
{
  "invocation_id": "abc123",
  "handler": "expense_approval",
  "status": "running",
  "total_steps": 5,
  "current_step": 2,
  "current_step_type": "AWAKEABLE:approval-xyz",
  "journal": [
    {"step_number": 0, "step_type": "HelloService.SayHello"},
    {"step_number": 1, "step_type": "AWAKEABLE:approval-xyz"}
  ],
  "workflow_state": {
    "cart": "[\"TICKET-001\",\"TICKET-002\"]"
  },
  "awakeables": {
    "approval-xyz": {
      "resolved": false,
      "created_at": 1234567890
    }
  }
}
```

**Run it:**
```bash
make run-query
```

**Use Cases:**
- Debugging workflow execution
- Monitoring workflow progress
- Building dashboards
- Detecting stuck workflows
- Viewing workflow state

---

## Example Summary

| Example | Description | Run Command |
|---------|-------------|-------------|
| `simple/` | Minimal 5-line example | `make run-simple` |
| `async/` | Fire-and-forget + result polling | `make run-async` |
| `typed/` | Type-safe workflows with generics | `make run-typed` |
| `local/` | Local in-process execution | `make run-local` |
| `multi-service/` | Multiple service calls in one workflow | `make run-multi-service` |
| `concurrent-workflows/` | N concurrent workflow invocations (configurable) | `make run-concurrent` |
| `retry-failures/` | Detect failed workflows and retry them | `make run-retry` |
| `pause-cancel/` | Pause/resume or cancel running workflows | `make run-pause` |
| `ticket-cart/` | Workflow state management (ctx.Get/Set) | `make run-ticket-cart` |
| `delayed-execution/` | Schedule workflows for future execution | `make run-delayed` |
| `approval-workflow/` | Awakeables for external callbacks | `make run-approval` |
| `query-api/` | Query workflow status via HTTP | `make run-query` |
| `restate-style/` | All patterns combined | `make run-restate` |

---

## Development Workflow

**Option 1: Local Development (Fast)**
```bash
# No NATS/services needed - everything runs in-process
go run examples/local/main.go
```

**Option 2: Full Distributed Setup**
```bash
# Terminal 1: Start NATS cluster
make docker-up

# Terminal 2: Start gRPC services
make run-service

# Terminal 3: Start processor
make run-processor

# Terminal 4: Run any example
make run-typed
make run-multi-service
make run-concurrent       # Default 10 concurrent invocations
./bin/concurrent-workflows --count 20  # Custom count
./bin/concurrent-workflows --count 10000 --timeout 600
make run-restate
```

---

## Key Concepts

### Durable Calls
Every `ctx.DurableCall()` is journaled:
```go
// First call
ctx.DurableCall("ServiceA", "Method1", req1, resp1)  // Journaled as step 1

// Second call
ctx.DurableCall("ServiceB", "Method2", req2, resp2)  // Journaled as step 2

// If workflow crashes here and replays, steps 1-2 are replayed from journal
// Only step 3 onwards will actually execute

// Third call
ctx.DurableCall("ServiceC", "Method3", req3, resp3)  // Journaled as step 3
```

### Exactly-Once Semantics
- Each workflow invocation has a unique ID
- Each durable call within a workflow is numbered
- On replay, completed steps return cached results
- Only failed/incomplete steps re-execute

### Type Safety
```go
// Workflow[TIn, TOut] provides compile-time guarantees
var Workflow = durable.NewWorkflow("name",
    func(ctx *durable.Context, input string) (int, error) {
        return 42, nil
    })

// Compiler enforces types:
result, _ := client.InvokeWorkflow[string, int](c, ctx, Workflow, "hello")
// result is type int, not interface{} or []byte
```

---

## Restate Feature Parity

This implementation achieves **complete feature parity** with [Restate](https://restate.dev):

| Feature | Restate | Our Implementation | Status |
|---------|---------|-------------------|--------|
| **Durable RPC Calls** | `ctx.rpc()` | `ctx.DurableCall()` | ✅ Complete |
| **Workflow State** | `ctx.get()` / `ctx.set()` | `ctx.Get()` / `ctx.Set()` | ✅ Complete |
| **Delayed Execution** | `ctx.sendDelayed()` | `ctx.SendDelayed()` | ✅ Complete |
| **Awakeables** | `ctx.awakeable()` | `ctx.Awakeable()` | ✅ Complete |
| **Journal Replay** | Deterministic replay | Journal-based replay | ✅ Complete |
| **Exactly-Once** | Per-step guarantees | Per-step guarantees | ✅ Complete |
| **Failure Recovery** | Automatic retry | Automatic retry | ✅ Complete |
| **Type Safety** | TypeScript generics | Go generics | ✅ Complete |

### What We Added Beyond Restate

1. **Configurable Concurrency** - Control max concurrent workflows via `--max-concurrent`
2. **Pause/Resume** - Temporarily halt workflows without losing state
3. **Cancellation** - Stop workflows permanently
4. **Query API** - HTTP API to inspect workflow state, journal, and awakeables
5. **NATS JetStream** - Built on battle-tested NATS for clustering and HA
6. **Local Development Mode** - Run workflows in-process for faster iteration

### Real-World Use Cases Enabled

✅ **E-commerce checkout flows** (cart management, payment webhooks, expiry timers)
✅ **Approval workflows** (expense approvals, PR reviews, manual interventions)
✅ **Payment processing** (Stripe webhooks, transaction tracking)
✅ **Saga patterns** (distributed transactions with compensation)
✅ **Scheduled tasks** (reminder emails, cleanup jobs)
✅ **Human-in-the-loop** (waiting for user input, external approvals)
✅ **Long-running jobs** (data processing, ETL pipelines)
✅ **Retry with backoff** (transient failure handling)
