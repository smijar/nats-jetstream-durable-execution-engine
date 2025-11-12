# Approval Workflow (Awakeables) Example

This example demonstrates `ctx.Awakeable`, a powerful feature for creating workflows that pause and wait for an external event before continuing. This is often used for "human-in-the-loop" processes.

The `ExpenseApprovalWorkflow` submits an expense and then suspends its own execution until a manager approves or rejects it by making an HTTP API call.

## How to Run

Ensure the NATS cluster, processor, and services are running first. The processor's HTTP API must be enabled (which it is by default).

```bash
# In separate terminals
make docker-up
make run-service
make run-processor
```

Then, run the example from the **root directory** of the project:

```bash
make run-approval
```

## Expected Output & Interaction

1.  **Client Submits Workflow:** The client will submit the workflow and then exit. The workflow is now durably suspended, waiting for the awakeable to be resolved.

    ```
    Running approval workflow example (awakeables)...
    ./bin/approval-workflow
    ========================================
    Expense Approval Workflow (Awakeables)
    ========================================

    📤 Submitting expense for approval...
    ...
    ⏳ Workflow submitted and waiting for approval...
    ```

2.  **Processor Waits:** In your `processor` terminal, you will see logs indicating that the workflow is waiting for an awakeable, along with the `curl` command needed to resolve it.

    ```
    ...
    ⏳ Waiting for manager approval (awakeable ID: approval-a1b2c3d4-e5f6)
       Manager can approve by calling:
       curl -X POST http://localhost:8080/api/awakeables/approval-a1b2c3d4-e5f6/resolve \
         -H 'Content-Type: application/json' \
         -d '{"result": "{\"approved\": true, \"comments\": \"Looks good!\"}"}'
    ...
    ```

3.  **External Approval:** Open a **new terminal** and run the `curl` command from the processor's logs to approve (or reject) the expense.

4.  **Workflow Resumes:** Once you send the `curl` command, the processor will receive the API call, resolve the awakeable, and the workflow will immediately resume execution from where it left off. You will see new logs in the `processor` terminal showing the final steps of the workflow.

## Key Features Demonstrated

*   **Awakeables:** Using `ctx.Awakeable` to suspend a workflow and wait for an external signal.
*   **Human-in-the-Loop:** A classic example of a workflow that requires manual intervention to proceed.
*   **External System Integration:** Shows how any external system that can make an HTTP request can interact with and control the flow of a durable workflow.
*   **Durable Suspension:** The workflow's state is safely persisted in the NATS KV store while it is suspended.

