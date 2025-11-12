# Direct gRPC Service Call Example

This example demonstrates how to call a gRPC service directly, bypassing the Durable Execution Engine entirely. This highlights that your gRPC services are standard, standalone applications that can be consumed by any gRPC client.

## When to Use a Direct Call vs. a Workflow

*   **Use a DIRECT CALL for:**
    *   **Simple, synchronous queries:** For example, a web server frontend that needs to quickly fetch user data and display it.
    *   **Low-latency requests:** A direct gRPC call has less overhead than a workflow invocation.
    *   **Stateless operations:** When the call is self-contained and you don't need to persist any state or guarantee its completion in the face of failures.

*   **Use a WORKFLOW CALL for:**
    *   **Reliability and Durability:** If the process making the call might crash, the workflow engine guarantees the call will eventually be made.
    *   **Multi-step business processes:** When you need to orchestrate a sequence of calls and ensure they all complete (e.g., process payment, update inventory, schedule shipping).
    *   **Long-running operations:** For processes that might take minutes, hours, or days.
    *   **Exactly-once execution:** The engine's journal prevents a step from being accidentally executed more than once on a retry.

Think of it like a **phone call vs. registered mail**. A direct gRPC call is a phone call: fast, immediate, but if the line drops, the message is lost. A workflow call is like sending registered mail: it has more overhead, but you have a durable, auditable guarantee that the message will be delivered.

## How to Run

Only the `HelloService` needs to be running for this example. The NATS cluster and the `processor` are **not** required.

1.  **Start the `HelloService`:**
    ```bash
    # In a separate terminal
    make run-service
    ```

2.  **Run the example:**
    ```bash
    # From the root directory of the project
    make run-direct-grpc
    ```

## Expected Output

The client will connect directly to the `HelloService`, make an RPC call, and print the response.

```
Running direct gRPC call example...
./bin/direct-grpc-call
========================================
Direct gRPC Service Call Example
========================================

Attempting to connect to HelloService at localhost:9090...
✅ Connected to gRPC server.
📞 Making SayHello RPC call with name: "Direct Client"
✅ Received response: Hello, Direct Client! Welcome to Durable Execution on NATS JetStream.

========================================
✓ Direct gRPC call demo complete!
========================================
```
