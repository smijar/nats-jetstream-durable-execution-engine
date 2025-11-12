# Architecture

This document provides a high-level overview of the architecture of the Durable Execution Engine.

## Overview

The Durable Execution Engine is a system that allows you to write reliable, long-running workflows that are resilient to failures. It is built on top of NATS JetStream and provides a simple, Go-based SDK for defining and executing workflows.

The core idea is to provide **durable execution** with **deterministic replay**. This means that workflows can survive process crashes, network failures, and service restarts, and will automatically resume from the last successfully completed step.

## Core Components

The system is composed of the following core components:

1.  **Client (`pkg/client`)**: The client provides a simple API for invoking workflows, checking their status, and managing their lifecycle (e.g., pausing, resuming, canceling).

2.  **Processor (`pkg/execution`)**: The processor is the heart of the engine. It is responsible for:
    *   Consuming workflow commands from the NATS JetStream.
    *   Loading the current state of a workflow.
    *   Replaying the journal of completed steps to restore the workflow's context.
    *   Executing the workflow handler.
    *   Saving the updated state and journal back to NATS.

3.  **Durable Context (`pkg/durable`)**: The `durable.Context` is an SDK that is passed to each workflow handler. It provides the following key functions:
    *   `DurableCall`: Makes a durable call to a gRPC service. The result of this call is automatically journaled, so it will not be re-executed on retry.
    *   `Get`/`Set`: Allows workflows to store and retrieve workflow-scoped state.
    *   `SendDelayed`: Schedules a workflow to be executed at a later time.
    *   `Awakeable`: Pauses a workflow until an external event occurs.

4.  **NATS JetStream (`pkg/jetstream`)**: NATS JetStream is used for both messaging and state persistence:
    *   **Command Stream**: A JetStream stream is used as a durable, ordered log of workflow commands.
    *   **State Store**: A JetStream Key-Value (KV) bucket is used to store the execution state of each workflow, including its journal.

5.  **gRPC Services (`cmd/services`, `cmd/ticket-service`)**: These are the business logic services that are invoked by workflows. They are standard gRPC services that are unaware of the durable execution engine.

## Execution Flow

The following diagram illustrates the execution flow of a workflow:

```
Client → Command Stream → Processor → Load State from KV
                                    ↓
                            Replay from Journal
                                    ↓
                            Execute Handler (durable.Context)
                                    ↓
                            Call gRPC Service
                                    ↓
                            Journal Result to KV
                                    ↓
                            Save Execution State
```

1.  The **Client** submits a workflow invocation command to the **Command Stream** in NATS JetStream.
2.  The **Processor** consumes the command from the stream.
3.  The **Processor** loads the current execution state for the given invocation ID from the **State Store** (KV bucket).
4.  The **Processor** replays the journal of completed steps, restoring the workflow's context without re-executing the steps.
5.  The **Processor** executes the workflow handler, passing it a `durable.Context`.
6.  The workflow handler uses the `durable.Context` to make a `DurableCall` to a gRPC service.
7.  The `durable.Context` journals the result of the gRPC call to the **State Store**.
8.  After the workflow handler completes, the **Processor** saves the final execution state.

## State Management

The state of each workflow execution is stored in a NATS KV bucket. The state is represented by the `ExecutionState` protobuf message, which contains:

*   `InvocationID`: A unique ID for the workflow execution.
*   `Handler`: The name of the workflow handler.
*   `Journal`: A list of `JournalEntry` messages, where each entry represents a completed step in the workflow.
*   `Status`: The current status of the workflow (e.g., `running`, `completed`, `failed`).
*   `WorkflowState`: A map of key-value pairs for storing workflow-scoped state.

## Key Guarantees

The architecture is designed to provide the following key guarantees:

*   **Durable Execution**: Workflows can survive failures and will be automatically resumed.
*   **Deterministic Replay**: The journal-based replay ensures that workflows are executed consistently across retries.
*   **Exactly-Once Semantics**: Each workflow invocation is guaranteed to be executed exactly once, thanks to the unique invocation ID and idempotent step execution.
*   **Fault Tolerance**: The system is resilient to process crashes, network failures, and service restarts.

## Orchestration Patterns: Sync and Async

In this engine, "synchronous" and "asynchronous" describe how a workflow orchestrates and waits for tasks.

### 1. Synchronous Calls (`ctx.DurableCall`)

This is the default and most common pattern, used for sequential tasks where each step depends on the result of the previous one.

*   **How it works:** When a workflow calls `ctx.DurableCall`, its execution **pauses** and waits for the remote gRPC service to return a result. The engine journals the result. On a retry, the engine returns the journaled result without re-calling the service.
*   **Analogy:** It behaves like a standard, blocking function call, but with the "superpowers" of durability and exactly-once execution.

**Inline Example:**
```go
func orderProcessingWorkflow(ctx *durable.Context) error {
    // Step 1: Synchronously validate the order. The workflow waits here.
    validateReq := &orderpb.ValidateRequest{OrderId: "order-123"}
    validateResp := &orderpb.ValidateResponse{}
    if err := ctx.DurableCall("OrderService", "Validate", validateReq, validateResp); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    // Step 2: This only runs after validation succeeds. The workflow waits again.
    paymentReq := &paymentpb.ChargeRequest{Amount: 99.99}
    paymentResp := &paymentpb.ChargeResponse{}
    if err := ctx.DurableCall("PaymentService", "Charge", paymentReq, paymentResp); err != nil {
        return fmt.Errorf("payment failed: %w", err)
    }

    return nil
}
```

### 2. Asynchronous "Fire-and-Forget" Calls (`ctx.SendDelayed`)

This pattern is used to kick off a separate, independent workflow without waiting for its result.

*   **How it works:** A workflow calls `ctx.SendDelayed` with a delay of `0`. This instructs the engine to immediately submit a *new* command to the NATS JetStream for a different workflow. The calling workflow does not pause; it continues its own execution immediately. The two workflows now run in parallel.
*   **Analogy:** It's like firing off a "send email" request and immediately moving on to the next task, without waiting to confirm the email was delivered.

**Inline Example:**
```go
func mainOrderWorkflow(ctx *durable.Context) error {
    // ... previous steps ...

    // Step 4: Synchronously confirm the order.
    ctx.DurableCall("OrderService", "Confirm", confirmReq, confirmResp)

    // Step 5: Asynchronously (fire-and-forget) start a separate workflow
    // to send a confirmation email. This call returns immediately.
    notificationInput := Notification{UserID: "user-123", Message: "Your order is confirmed!"}
    ctx.SendDelayed("send-confirmation-email-workflow", notificationInput, 0, "user-123")

    // Step 6: The main workflow continues to the next step without waiting for the email.
    ctx.DurableCall("ShippingService", "SchedulePickup", shipReq, shipResp)

    return nil
}
```

### 3. Asynchronous Calls with External Systems (`ctx.Awakeable`)

This pattern is for pausing a workflow to wait for a signal from a system outside the engine, such as a human action or a third-party webhook.

*   **How it works:** The workflow generates a unique `awakeableID` and calls `ctx.Awakeable()`. This suspends the workflow. An external system can then make an HTTP API call to the processor, providing the `awakeableID` and a result. This action "wakes up" the workflow, which resumes execution with the provided result.
*   **Analogy:** It's like pausing your work to wait for a package delivery. You can't proceed until the delivery person (the external system) rings your doorbell (the API call).

**Inline Example:**
```go
var ExpenseApprovalWorkflow = durable.NewWorkflow("expense_approval",
    func(ctx *durable.Context, request ExpenseRequest) (string, error) {
        // ... log the request ...

        // Step 2: Create an awakeable and suspend the workflow.
        // The workflow will not proceed past this line until the API is called.
        awakeableID := fmt.Sprintf("approval-%s", uuid.New().String())
        log.Printf("⏳ Waiting for manager approval (ID: %s)", awakeableID)
        resultBytes, err := ctx.Awakeable(awakeableID)
        if err != nil {
            // This block runs if the awakeable was rejected via the API
            return "Rejected", err
        }

        // Step 3: The workflow resumes here after the awakeable is resolved.
        var approval ApprovalResult
        json.Unmarshal(resultBytes, &approval)
        if approval.Approved {
            return "✅ Approved!", nil
        }

        return "❌ Rejected.", nil
    })
```

## Workflow Lifecycle Control: Detailed Explanation

The system provides comprehensive workflow lifecycle management with pause, resume, and cancel capabilities, leveraging NATS JetStream's message acknowledgment mechanisms and the workflow's `ExecutionState` in the KV store.

### How Pause & Resume Works

1.  **Client Initiates Pause:** The client updates the workflow's `ExecutionState` in the NATS KV store, changing its `Status` field from `"running"` to `"paused"`.
    ```go
    // Client pauses a running workflow
    err := client.Pause(ctx, invocationID)
    ```
2.  **Processor Reacts:** The processor loads the state, sees `Status == "paused"`, and **negatively acknowledges (Nak)** the message. This tells NATS JetStream to redeliver it later.

3.  **Holding Pattern:** The processor continues to receive and `Nak` the message, effectively pausing execution.

4.  **Client Initiates Resume:** The client updates the `Status` in the KV store back to `"running"`.
    ```go
    // Client resumes a paused workflow
    err := client.Resume(ctx, invocationID)
    ```
5.  **Processor Continues:** On the next redelivery, the processor sees the `"running"` status and resumes normal execution from the journal.

### How Cancel Works

1.  **Client Initiates Cancel:** The client updates the `ExecutionState` in the NATS KV store, setting the `Status` to `"cancelled"`.
    ```go
    // Client cancels a running or paused workflow
    err := client.Cancel(ctx, invocationID)
    ```
2.  **Processor Reacts:** The processor loads the state and sees `Status == "cancelled"`.

3.  **Termination Mechanism:** Instead of a `Nak`, the processor **successfully acknowledges (Ack)** the message. This tells NATS JetStream that it is done with the message.

4.  **Permanent Stop:** Because the command message is removed from the stream, the processor will **never receive it again**. The workflow is permanently stopped.
