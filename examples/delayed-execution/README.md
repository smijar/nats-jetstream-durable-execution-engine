# Delayed Execution Example

This example demonstrates how to schedule a workflow to run at a future time using `ctx.SendDelayed`.

The main `scheduler` workflow runs immediately and its only job is to submit a *new* command for a different workflow (`delayed_greeting`) to be executed after a specified delay.

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
make run-delayed
```

## Expected Output

The client will immediately print a confirmation that the delayed workflow has been scheduled.

```
Running delayed execution example...
./bin/delayed-execution
========================================
Delayed Execution Example
========================================

📤 Scheduling a delayed greeting (30 seconds)...

✅ Result: Scheduled greeting for 'Delayed Hello World' to execute in 30 seconds

========================================
✓ Scheduler workflow completed!
========================================

The delayed greeting workflow will execute in 30 seconds.
Check the processor logs to see when it runs!
```

After 30 seconds, you will see logs in your `processor` terminal indicating that the `delayed_greeting` workflow has started and completed, showing that the NATS JetStream message was delivered and processed after the specified delay.

## Key Features Demonstrated

*   **Delayed Invocation:** Using `ctx.SendDelayed` to schedule a new workflow invocation for a future time.
*   **Message Scheduling:** This works by publishing a NATS message with a `Nats-Scheduled-Time` header. The processor is designed to `Nak` (negatively acknowledge) messages that are not yet ready for execution, causing JetStream to redeliver them later.
*   **Use Cases:** This pattern is ideal for implementing timeouts, reminders, scheduled tasks, or retry logic with exponential backoff.
