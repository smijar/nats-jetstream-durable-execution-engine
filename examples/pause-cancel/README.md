# Pause, Resume, and Cancel Example

This example demonstrates the workflow lifecycle control features of the engine. It shows how a client can externally pause, resume, and cancel a running workflow.

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
make run-pause
```

## Expected Output

The demo runs in two parts:

1.  **Pause & Resume:** It starts a workflow, pauses it midway through, waits a few seconds, and then resumes it to completion.
2.  **Cancel:** It starts a second workflow and cancels it permanently before it can finish.

```
Running pause/cancel example...
./bin/pause-cancel
============================================================
Workflow Pause/Resume & Cancel Demo
============================================================

📋 DEMO 1: Pause & Resume
------------------------------------------------------------
✓ Submitted workflow (invocation_id: ...)

⏳ Letting workflow run for 1 second...

⏸️  PAUSING workflow...
✓ Workflow paused

Status check: paused
Journal entries: 3

💤 Workflow is paused... waiting 2 seconds...

▶️  RESUMING workflow...
✓ Workflow resumed

⏳ Waiting for workflow to complete...

✅ Final Status: pending
Journal entries: 3

============================================================

📋 DEMO 2: Cancel Workflow
------------------------------------------------------------
✓ Submitted workflow (invocation_id: ...)

⏳ Letting workflow run for 1 second...

🛑 CANCELLING workflow...
✓ Workflow cancelled

✅ Final Status: cancelled
Journal entries completed before cancellation: 3

============================================================
✓ Demo completed!
============================================================
```

## Key Features Demonstrated

*   **Pause:** Using `client.Pause` to temporarily suspend a workflow's execution.
*   **Resume:** Using `client.Resume` to continue a paused workflow from where it left off.
*   **Cancel:** Using `client.Cancel` to permanently terminate a workflow.
*   **State-Based Control:** This demonstrates how the client controls the workflow's lifecycle by updating its `Status` in the NATS KV store, which the processor then reacts to.
