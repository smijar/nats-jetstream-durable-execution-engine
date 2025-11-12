# Restate-Style Durable Execution API

We've implemented a **Restate-inspired** type-safe API for durable execution workflows, combining workflow definition and invocation in a unified, ergonomic interface.

## Overview

The Restate-style API brings three major improvements:

1. **Type Safety** - Workflows are generic functions with compile-time type checking
2. **Co-location** - Handler definitions live alongside invocation code
3. **Flexible Deployment** - Same code runs locally (dev) or distributed (production)

## Quick Example

```go
// Define a type-safe workflow
var HelloWorkflow = durable.NewWorkflow("greeting",
    func(ctx *durable.Context, name string) (string, error) {
        req := &hellopb.HelloRequest{Name: name}
        resp := &hellopb.HelloResponse{}

        ctx.DurableCall("HelloService", "SayHello", req, resp)

        return resp.Message, nil
    })

// Invoke with full type safety
c, _ := client.NewClient("nats://127.0.0.1:4322")
result, _ := client.InvokeWorkflow[string, string](c, ctx, HelloWorkflow, "World")

fmt.Println(result) // Fully typed as string!
```

## Architecture

### 1. Typed Workflows

```go
type Workflow[TIn, TOut any] struct {
    name    string
    handler func(*Context, TIn) (TOut, error)
}
```

**Benefits:**
- Input and output types are enforced at compile time
- IDE autocomplete for workflow functions
- Refactoring is safe - compiler catches type mismatches

**Example:**
```go
// String in, string out
var GreetingWorkflow = durable.NewWorkflow("greet",
    func(ctx *durable.Context, name string) (string, error) {
        // Implementation
        return "Hello, " + name, nil
    })

// Struct in, struct out
type OrderRequest struct { OrderID string; Items []string }
type OrderResult struct { OrderID string; Total float64 }

var ProcessOrderWorkflow = durable.NewWorkflow("process_order",
    func(ctx *durable.Context, req OrderRequest) (OrderResult, error) {
        // Implementation
        return OrderResult{OrderID: req.OrderID, Total: 99.99}, nil
    })
```

### 2. Workflow Registry

Workflows are registered in a central registry:

```go
registry := durable.NewWorkflowRegistry()
registry.Register(HelloWorkflow)
registry.Register(ProcessOrderWorkflow)
```

The registry:
- Adapts typed workflows to untyped `HandlerFunc`
- Manages handler lookup by name
- Supports both typed and legacy handlers

### 3. Client API

Three modes of operation:

#### A. Remote Invocation (Production)

```go
c, _ := client.NewClient("nats://127.0.0.1:4322")

// Invoke workflow - runs on processor cluster
result, _ := client.InvokeWorkflow[string, string](
    c, ctx, HelloWorkflow, "Production")
```

Workflow executes on remote processor, results returned via NATS.

#### B. Local Development Mode

```go
c, _ := client.NewClient("nats://127.0.0.1:4322")

// Register workflows for local execution
c.Register(HelloWorkflow)
c.Register(ProcessOrderWorkflow)

// Start serving in background
go c.ServeHandlers(ctx)

// Now invocations run locally!
result, _ := client.InvokeWorkflow[string, string](
    c, ctx, HelloWorkflow, "Local")
```

**Perfect for:**
- Development and testing
- Debugging workflows
- Fast iteration

#### C. Workflow References (Client-only)

```go
// Client doesn't need handler implementation
workflowRef := HelloWorkflow.Ref()

// Invoke remote workflow by reference
result, _ := client.InvokeWorkflow[string, string](
    c, ctx, workflowRef, "Remote")
```

**Use case:** Microservices architecture where Team A writes workflows, Team B invokes them.

## Deployment Patterns

### Pattern 1: Monolith (Simplest)

Everything in one process:

```go
func main() {
    c, _ := client.NewClient("nats://...")

    // Register handlers
    c.Register(Workflow1)
    c.Register(Workflow2)

    // Serve locally
    go c.ServeHandlers(ctx)

    // Invoke
    result, _ := client.InvokeWorkflow[In, Out](c, ctx, Workflow1, input)
}
```

**Pros:** Simple, fast, easy to debug
**Cons:** No horizontal scaling

### Pattern 2: Distributed (Production)

Separate client and processor:

**Processor:**
```go
processor := execution.NewProcessor(jsClient)
processor.RegisterHandler("workflow1", handler1)
processor.RegisterHandler("workflow2", handler2)
processor.Start(ctx, "processor-1")
```

**Client:**
```go
c, _ := client.NewClient("nats://...")

// Reference only - no handler code needed
workflowRef := Workflow1.Ref()

result, _ := client.InvokeWorkflow[In, Out](c, ctx, workflowRef, input)
```

**Pros:** Horizontal scaling, separation of concerns
**Cons:** More complex deployment

### Pattern 3: Hybrid

Local for development, distributed for production:

```go
func main() {
    c, _ := client.NewClient("nats://...")

    if os.Getenv("ENV") == "development" {
        // Local mode
        c.Register(AllWorkflows...)
        go c.ServeHandlers(ctx)
    }

    // Same invocation code works in both modes!
    result, _ := client.InvokeWorkflow[In, Out](c, ctx, workflow, input)
}
```

## Comparison with Restate

| Feature | Restate | Our Implementation |
|---------|---------|-------------------|
| Type Safety | ✅ TypeScript/Java generics | ✅ Go generics |
| Handler Registration | ✅ `restate.service()` | ✅ `durable.NewWorkflow()` |
| Local Execution | ✅ Development mode | ✅ `ServeHandlers()` |
| Remote Invocation | ✅ `ctx.run()` | ✅ `InvokeWorkflow()` |
| Workflow References | ✅ Service proxies | ✅ `workflow.Ref()` |
| Deterministic Replay | ✅ Journal-based | ✅ Journal-based |
| Durable Storage | ✅ RocksDB | ✅ NATS KV |
| Message Queue | ✅ Kafka/NATS | ✅ NATS JetStream |

## Examples

See the `examples/` directory:

- **`examples/typed/`** - Basic type-safe invocation
- **`examples/local/`** - Local development mode
- **`examples/restate-style/`** - Comprehensive showcase of all features

Run them:
```bash
make run-typed
make run-local
make run-restate
```

## API Reference

### Defining Workflows

```go
durable.NewWorkflow[TIn, TOut](
    name string,
    handler func(*Context, TIn) (TOut, error)
) *Workflow[TIn, TOut]
```

### Registering Workflows

```go
client.Register(workflow interface{})
```

### Invoking Workflows

```go
client.InvokeWorkflow[TIn, TOut](
    c *Client,
    ctx context.Context,
    workflow interface{}, // *Workflow[TIn, TOut] or *WorkflowRef[TIn, TOut]
    input TIn
) (TOut, error)
```

### Local Serving

```go
client.ServeHandlers(ctx context.Context) error
```

### Workflow References

```go
workflow.Ref() *WorkflowRef[TIn, TOut]
```

## Limitations & Future Work

**Current Limitations:**
1. Generic methods not allowed in Go - using standalone `InvokeWorkflow()` function
2. Type parameters must be explicit: `InvokeWorkflow[string, string](...)`
3. Output marshaling uses JSON for simple types (protobuf for proto.Message)

**Future Enhancements:**
1. Type inference - detect types from workflow definition
2. Workflow composition - workflows calling workflows with type safety
3. Generated client stubs - like gRPC, generate type-safe clients
4. Processor integration - register typed workflows directly with processor

## Migration from Legacy API

**Before (Legacy):**
```go
result, _ := c.Invoke(ctx, "hello_workflow")
var resp hellopb.HelloResponse
result.GetFirstResponse(&resp)
fmt.Println(resp.Message)
```

**After (Restate-style):**
```go
result, _ := client.InvokeWorkflow[string, string](
    c, ctx, HelloWorkflow, "World")
fmt.Println(result) // Already a string!
```

**Benefits:**
- No manual response extraction
- Compile-time type checking
- Better IDE support

## Conclusion

The Restate-style API brings modern, type-safe workflow programming to our durable execution engine:

✅ **Developer Experience** - Write once, run anywhere (local or distributed)
✅ **Type Safety** - Catch errors at compile time, not runtime
✅ **Flexibility** - Start simple, scale when needed
✅ **Compatibility** - Works alongside legacy API

This is a **significant improvement** in usability while maintaining all the power of durable execution!
