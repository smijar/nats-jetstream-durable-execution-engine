# Ticket Cart Example

This example demonstrates a more complex, stateful workflow that simulates adding tickets to a shopping cart. It showcases how to manage workflow-scoped state that persists across multiple invocations.

## How to Run

This workflow calls the `TicketService`, so you must run it in addition to the other standard services.

```bash
# In separate terminals
make docker-up
make run-service
make run-ticket-service
make run-processor
```

Then, run the example from the **root directory** of the project:

```bash
make run-ticket-cart
```

## Expected Output

The example runs through three scenarios:
1.  A user adds their first ticket to a new cart.
2.  The same user adds a second ticket to their existing cart.
3.  A different user tries to add a ticket that has already been reserved by the first user, which fails as expected.

```
Running ticket cart example (Restate-style)...
./bin/ticket-cart
========================================
Ticket Cart Example (Restate-style)
========================================

📤 Scenario 1: Adding first ticket to cart...

✅ Result: success=true

📤 Scenario 2: Adding second ticket to same cart...

✅ Result: success=true

📤 Scenario 3: Trying to add already reserved ticket...

✅ Result: success=false (expected: false)

========================================
✓ Test completed!
========================================
```

## Key Features Demonstrated

*   **Workflow State:** Using `ctx.Get("cart", &cart)` and `ctx.Set("cart", cart)` to manage state that is scoped to a specific workflow instance (in this case, the user's cart).
*   **Stateful Logic:** The workflow's behavior changes based on the current state of the cart.
*   **Partitioning:** The `partitionKey` is set to the `userID`, ensuring that all operations for a single user's cart are processed sequentially by the same consumer, preventing race conditions.
*   **Delayed Execution:** It uses `ctx.SendDelayed` to schedule a separate `expire_ticket` workflow, demonstrating how to implement timeouts or expirations.
