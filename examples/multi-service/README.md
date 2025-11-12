# Multi-Service Invocation Example

This example demonstrates a workflow that makes multiple, sequential `DurableCall`s to gRPC services.

Each call is journaled as a separate step. If the workflow were to fail midway through (e.g., after the second call), upon retry it would replay the results of the first two calls from its journal and only re-execute the third call.

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
make run-multi-service
```

## Expected Output

The example first runs the workflow remotely via NATS, then runs it again in local development mode. Both executions produce a summary of the four service calls.

```
Running multi-service invocation example...
./bin/multi-service
========================================
Multi-Service Invocation Demo
========================================

Example 1: Remote Execution
----------------------------
Multi-Service Workflow Results:
1️⃣  Hello, Durable Execution (English)! Welcome to Durable Execution on NATS JetStream.
2️⃣  Hello, Durable Execution (Spanish)! Welcome to Durable Execution on NATS JetStream.
3️⃣  Hello, Durable Execution (French)! Welcome to Durable Execution on NATS JetStream.
📊 Hello, Summary! Welcome to Durable Execution on NATS JetStream.
Total service calls: 4

Example 2: Local Execution
----------------------------
Multi-Service Workflow Results:
1️⃣  Hello, Local Development (English)! Welcome to Durable Execution on NATS JetStream.
2️⃣  Hello, Local Development (Spanish)! Welcome to Durable Execution on NATS JetStream.
3️⃣  Hello, Local Development (French)! Welcome to Durable Execution on NATS JetStream.
📊 Hello, Summary! Welcome to Durable Execution on NATS JetStream.
Total service calls: 4

========================================
✓ All examples completed!
========================================
```

## Key Features Demonstrated

*   **Multi-Step Workflows:** Shows how to compose a workflow from multiple sequential `DurableCall`s.
*   **Deterministic Replay:** Illustrates the core principle that each `DurableCall` is an atomic, resumable step.
*   **Workflow Composition:** Provides a basic example of building a more complex process from smaller service calls.
