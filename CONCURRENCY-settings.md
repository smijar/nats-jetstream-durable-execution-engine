
  What is MaxAckPending?

  MaxAckPending is a NATS JetStream consumer setting that controls the maximum number of messages that can be in-flight (delivered but not yet acknowledged) at any
  given time.

  How It Works

  NATS Stream: [Msg1] [Msg2] [Msg3] [Msg4] [Msg5] [Msg6] ...
                  ↓      ↓      ↓
  Consumer with MaxAckPending=3:
               [Processing Msg1]
               [Processing Msg2]
               [Processing Msg3]

  NATS: "I've sent 3 messages, won't send Msg4 until one is ACKed"

  Once 3 messages are delivered and awaiting acknowledgment, NATS stops delivering new messages until at least one is acknowledged.

  How It Affects the Processor

  Looking at pkg/execution/processor.go:56-64:

  consumer.Consume(func(msg natsjs.Msg) {
      if err := p.processMessage(ctx, msg); err != nil {
          log.Printf("Error processing message: %v", err)
          msg.NakWithDelay(5 * time.Second)
      } else {
          msg.Ack()  // <-- Frees up one slot in MaxAckPending
      }
  })

  The Flow:

  1. NATS delivers message → MaxAckPending counter increments
  2. Processor receives message → Starts executing workflow in a goroutine
  3. Workflow runs → Could take seconds/minutes (calling gRPC services, etc.)
  4. Workflow completes → msg.Ack() called → MaxAckPending counter decrements
  5. NATS delivers next message → Process repeats

  Impact on Go Concurrency

  Current Behavior (No Explicit MaxAckPending)

  In pkg/jetstream/client.go:127-135, we don't set MaxAckPending:

  consumer, err := c.js.CreateOrUpdateConsumer(ctx, CommandStreamName, natsjs.ConsumerConfig{
      Name:          consumerName,
      AckPolicy:     natsjs.AckExplicitPolicy,
      AckWait:       30 * time.Second,  // How long to wait for ACK
      MaxDeliver:    3,                  // Max redelivery attempts
      // MaxAckPending: NOT SET (defaults to ~1000)
  })

  Default behavior: NATS allows ~1000 messages in-flight simultaneously.

  What This Means for Go:

  The Consume() callback (processMessage) is called in a goroutine for each message. So with default MaxAckPending:

  Message 1 → Goroutine 1 → Workflow Execution (blocking on gRPC calls)
  Message 2 → Goroutine 2 → Workflow Execution (blocking on gRPC calls)
  Message 3 → Goroutine 3 → Workflow Execution (blocking on gRPC calls)
  ...
  Message 1000 → Goroutine 1000 → Workflow Execution

  Result: Up to ~1000 concurrent goroutines executing workflows simultaneously!

  Why This Matters

  Memory Impact:

  - Each goroutine uses ~2KB minimum stack space
  - Each workflow execution holds:
    - gRPC connections
    - Request/response buffers
    - Journal entries in memory
    - State objects

  1000 concurrent workflows could easily consume:
  - 1000 goroutines × 2KB = 2MB (just stacks)
  - 1000 workflows × gRPC overhead = 100MB-500MB (connections, buffers)
  - Total: ~500MB-1GB depending on workflow complexity

  CPU Impact:

  - Each workflow making gRPC calls (I/O-bound, but still CPU for marshaling/unmarshaling)
  - Context switching overhead with 1000+ goroutines
  - GC pressure from allocations

  Downstream Service Impact:

  - If each workflow calls HelloService.SayHello(), you're making 1000 concurrent gRPC calls to the service
  - Could overwhelm the service or hit rate limits

  Example Scenarios

  Scenario 1: MaxAckPending = 10 (Low)

  ✅ Benefits:
    - Only 10 workflows executing concurrently
    - Low memory usage (~20MB)
    - Controlled load on downstream services

  ❌ Drawbacks:
    - Slower throughput
    - If workflows are fast, processor is underutilized

  Scenario 2: MaxAckPending = 500 (Medium)

  ✅ Benefits:
    - Good balance of throughput and resource usage
    - ~250MB memory usage
    - Reasonable load on services

  ❌ Drawbacks:
    - Still significant concurrent load

  Scenario 3: MaxAckPending = 5000 (High)

  ✅ Benefits:
    - Maximum throughput
    - Processes many workflows in parallel

  ❌ Drawbacks:
    - High memory usage (1GB+)
    - Could overwhelm downstream services
    - More GC pressure

  Relationship to AckWait

  Looking at line 131: AckWait: 30 * time.Second

  This is timeout per message. If a workflow takes longer than 30s without ACK:
  - NATS considers it "failed"
  - NATS redelivers the message (up to MaxDeliver: 3 times)
  - The goroutine might still be running (causing duplicate execution!)

  With paused workflows, we intentionally NAK messages (msg.NakWithDelay(5s)), so they're redelivered later when resumed.

  Summary

  MaxAckPending is a concurrency throttle that:
  1. Limits in-flight messages from NATS to the processor
  2. Controls goroutine count (one per message being processed)
  3. Manages resource usage (memory, CPU, connections)
  4. Protects downstream services from being overwhelmed

  Without setting it: You get NATS default (~1000), which might be too high for resource-constrained environments or could overwhelm services.

  By making it configurable: Users can tune it based on:
  - Available memory/CPU
  - Downstream service capacity
  - Desired throughput
  - Workflow complexity (fast vs slow workflows)

  Should I proceed with adding a --max-concurrent flag to make MaxAckPending configurable?

  ----


  1. --max-concurrent Flag (Controls MaxAckPending)

  - Default: 100 concurrent workflows
  - What it does: Limits how many workflow messages NATS delivers simultaneously
  - Impact: Controls memory usage, CPU load, and downstream service pressure

  2. --ack-wait Flag (Controls AckWait timeout)

  - Default: 300 seconds (5 minutes)
  - What it does: How long NATS waits for a workflow to complete before considering it "timed out" and redelivering
  - Impact: Allows longer-running workflows without redelivery

  📁 Files Modified

  1. cmd/processor/main.go: Added flag parsing and passed values to processor
  2. pkg/execution/processor.go: Updated Start() method signature to accept parameters
  3. pkg/jetstream/client.go: Updated SubscribeToCommands() to accept and use MaxAckPending and AckWait
  4. README.md: Added documentation for new flags with usage examples

  🚀 Usage Examples

  # Default settings (100 concurrent, 5min timeout)
  ./bin/processor

  # Low resource usage (10 concurrent, 2min timeout)
  ./bin/processor --max-concurrent 10 --ack-wait 120

  # High throughput (500 concurrent, 10min timeout)
  ./bin/processor --max-concurrent 500 --ack-wait 600

  # For long-running workflows (50 concurrent, 30min timeout)
  ./bin/processor --max-concurrent 50 --ack-wait 1800

  🎯 How to Choose Values

  --max-concurrent:
  - 10-50: Resource-constrained (1-2GB RAM), low traffic
  - 100-200: Balanced production (4-8GB RAM), moderate traffic
  - 500-1000: High throughput (16GB+ RAM), heavy traffic

  --ack-wait:
  - 120s (2min): Fast workflows (simple API calls)
  - 300s (5min): Standard workflows (multiple service calls)
  - 600-1800s (10-30min): Long-running workflows (batch processing, ML inference)

  The processor now logs these settings on startup:
  2025-11-11 12:00:00 Starting Durable Execution Processor...
  2025-11-11 12:00:00 Configuration: max-concurrent=100, ack-wait=5m0s
  2025-11-11 12:00:00 Processor started, consuming from processor-1 (max-concurrent=100, ack-wait=5m0s)

  Everything is built and ready to use! 🎉
