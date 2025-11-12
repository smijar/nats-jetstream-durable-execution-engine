# Retry Failures Example

This example demonstrates how to detect failed workflows and retry them.

It submits a mix of workflows that are designed to succeed and fail. It then checks the status of each one, identifies the failures, and resubmits them with modified input to ensure they succeed on the second attempt.

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
make run-retry
```

## Expected Output

The output shows the five phases of the demo:
1.  Submitting a mix of successful and "flaky" workflows.
2.  Checking the status and identifying that the flaky ones have failed.
3.  (Phase 3 is skipped in the code).
4.  Resubmitting the failed workflows with a new input that allows them to succeed.
5.  Verifying that the retried workflows completed successfully.

```
Running retry failures example...
./bin/retry-failures
========================================
Workflow Failure Detection & Retry Demo
========================================

📤 PHASE 1: Submitting workflows...
-----------------------------------
  ✓ Submitted: task-success-1 (invocation_id: ...)
  ✓ Submitted: task-flaky-1 (invocation_id: ...)
  ...

📊 PHASE 2: Checking workflow status...
---------------------------------------
  ...
  Workflow: task-flaky-1
    Status: failed
    ❌ FAILED
  ...

🔄 PHASE 4: Retrying failed workflows...
----------------------------------------
  Retrying: task-flaky-1 → task-flaky-1-retry
    ✓ Resubmitted (invocation_id: ...)
  ...

✅ PHASE 5: Verifying retry results...
--------------------------------------
  Retry: task-flaky-1-retry
    Status: completed
    ✅ RETRY SUCCEEDED!
  ...

========================================
✓ Demo completed!
========================================
```

## Key Features Demonstrated

*   **Failure Detection:** Shows how to use `client.GetResult` to check the `Status` of a workflow and identify failures.
*   **Retry Logic:** Demonstrates a client-side retry pattern where a failed workflow is invoked again with a new `invocation_id`.
*   **Idempotency:** The `flaky_workflow` is designed to fail on its first run but succeed on a retry, showcasing how workflows can be designed for idempotency.
