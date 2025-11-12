# Type-Safe Workflow Example

This example demonstrates the Restate-style, type-safe API for defining and invoking workflows. It provides compile-time guarantees for workflow inputs and outputs, reducing the need for manual marshaling and unmarshaling.

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
make run-typed
```

## Expected Output

The client will invoke the workflow with a strongly-typed input (`HelloWorkflowInput`) and receive a strongly-typed output (`string`), which it then prints.

```
Running typed workflow example...
./bin/typed
Result: Hello, TypeSafe World! Welcome to Durable Execution on NATS JetStream.
```

## Key Features Demonstrated

*   **Type-Safe Workflows:** Defining a workflow with generic input and output types using `durable.NewWorkflow`.
*   **Type-Safe Invocation:** Invoking the workflow with compile-time type checking using `client.InvokeWorkflow`.
*   **Automatic Marshaling:** The framework automatically handles the marshaling (e.g., to JSON) of inputs and outputs.
