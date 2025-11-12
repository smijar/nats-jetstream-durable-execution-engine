# Local Execution Example

This example demonstrates how to run a workflow in a local, in-process development mode.

In this mode, the workflow logic is executed directly within the client's process, bypassing NATS for command submission. This provides a faster feedback loop for development and testing, as you don't need to run the main `processor`.

## How to Run

You only need to run the gRPC service that the workflow calls. The NATS cluster and the main processor are **not** required for this example.

1.  **Start the gRPC service:**
    ```bash
    # In a separate terminal
    make run-service
    ```

2.  **Run the example:**
    ```bash
    # From the root directory of the project
    make run-local
    ```

## Expected Output

The client will register the workflow handler, serve it locally, and execute the workflow in-process, printing the final result.

```
Running local development example...
./bin/local
✓ Executed locally!
Result: Hello, Local World! Welcome to Durable Execution on NATS JetStream.
```

## Key Features Demonstrated

*   **Local Development Mode:** Shows how to use `c.Register()` and `c.ServeHandlers()` to run workflows locally.
*   **In-Process Execution:** The workflow is executed as a simple function call within the client, making debugging easier.
*   **Service Registry on Client:** Demonstrates the need to provide a `ServiceRegistry` to the client (`c.SetServiceRegistry()`) so it knows how to call the required gRPC services during local execution.
