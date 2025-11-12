# Concurrent Workflows Example

This example demonstrates the engine's ability to handle many workflow invocations concurrently. It submits a configurable number of workflows in parallel and waits for them all to complete.

This is useful for load testing and verifying the scalability and correctness of the system under concurrent load.

## How to Run

Ensure the NATS cluster, processor, and services are running first:

```bash
# In separate terminals
make docker-up
make run-service
make run-processor
```

Then, run the example from the **root directory** of the project:

```bash
make run-concurrent
```

You can also run the compiled binary with flags to change the number of concurrent workflows:

```bash
# Build first if you haven't already
make build

# Run 50 concurrent workflows
./bin/concurrent-workflows --count 50
```

## Expected Output

The client will submit 10 workflows (by default) and print a summary of the results, including the success/failure count and the average time per workflow.

```
Running concurrent workflows example...
./bin/concurrent-workflows
========================================
Concurrent Workflow Invocations (N=10)
Timeout: 5m0s per workflow
========================================

🚀 Submitting 10 workflow invocations...


========================================
Results Summary
========================================

AsyncWorkflow[user-006] completed:
  Step1: Hello, User:user-006! Welcome to Durable Execution on NATS JetStream.
  Step2: Hello, Async:user-006! Welcome to Durable Execution on NATS JetStream.
  Step3: Hello, Confirm:user-006! Welcome to Durable Execution on NATS JetStream.

... (more results) ...

Total Invocations: 10
✅ Success: 10
❌ Failed: 0
⏱️  Total Time: 506.949584ms
📊 Avg Time per Workflow: 50.694958ms

========================================
✓ Test completed!
========================================
```

## Key Features Demonstrated

*   **Scalability:** Shows the engine's ability to process a high volume of workflows in parallel.
*   **Concurrency Control:** The processor's `--max-concurrent` flag controls how many workflows it will execute at once, providing backpressure.
*   **Partitioning:** Although not explicitly configured here, using different partition keys (`client.WithPartitionKey`) would allow different processors to handle subsets of these concurrent workflows.
