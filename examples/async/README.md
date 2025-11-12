# Async Invocation Example

This example demonstrates the "fire-and-forget" pattern for invoking a workflow. The client submits the workflow and immediately proceeds with other work, checking the result later.

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
make run-async
```

## Expected Output

The client will submit the workflow and print its `invocation_id`. It will then simulate doing other work for a couple of seconds before polling for the final result.

```
Running async example...
./bin/async
Submitted workflow: 1d0f75aa-f15f-4812-b9db-4fe6b172ddec
Doing other work while workflow executes...
Result: Hello, ! Welcome to Durable Execution on NATS JetStream. (status: completed)
```

## Key Features Demonstrated

*   **Async Invocation:** Shows how to start a workflow without blocking using `client.InvokeAsync`.
*   **Result Polling:** Demonstrates how to retrieve the result of a workflow later using `client.GetResult` with the `invocation_id`.
