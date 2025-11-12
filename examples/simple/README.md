# Simple Example

This example demonstrates the most basic usage of the Durable Execution client. It invokes a simple `hello_workflow` and prints the result.

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
make run-simple
```

## Expected Output

The client will submit the workflow and wait for its completion, then print the final message from the `HelloService`.

```
Running simple example...
./bin/simple
Hello, Simple Client! Welcome to Durable Execution on NATS JetStream.
```

## Key Features Demonstrated

*   **Basic Invocation:** Shows the simplest way to invoke a workflow using `client.Invoke`.
*   **Durable Call:** The workflow itself makes a `ctx.DurableCall` to a gRPC service.
*   **Result Handling:** Shows how to get the final result from the workflow execution.
