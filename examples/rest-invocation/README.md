# REST Invocation Example

This example demonstrates how to invoke and manage workflows using the HTTP/REST API.

This is useful for clients written in languages other than Go, or for simple integrations using tools like `curl`.

## Prerequisites

1.  **Start the services:** Make sure the NATS cluster, the `processor`, and the `services` are all running. The `processor` must be running with the HTTP API enabled (which it is by default).

    ```bash
    # In separate terminals
    make docker-up
    make run-service
    make run-processor
    ```

## How to Use

Open a new terminal to run the following `curl` commands.

### 1. Invoke a Workflow

We will invoke the `hello_workflow`, which is registered by the processor.

*   **Endpoint:** `POST /api/workflows/hello_workflow/invoke`
*   **Body:** A JSON object with the input for the workflow. The `hello_workflow` expects an object with a `name` field.

```bash
curl -X POST http://localhost:8080/api/workflows/hello_workflow/invoke \
  -H 'Content-Type: application/json' \
  -d '{"name": "REST Client"}'
```

**Expected Response:**

You will receive a JSON response containing the unique `invocation_id` for this workflow execution.

```json
{
  "invocation_id": "a1b2c3d4-e5f6-7890-1234-567890abcdef"
}
```

Copy this `invocation_id` for the next step.

### 2. Check the Workflow Status

Using the `invocation_id` from the previous step, you can query the status of the workflow.

*   **Endpoint:** `GET /api/workflows/{invocation_id}`

Replace `{invocation_id}` with the ID you received.

```bash
# Replace with your actual invocation ID
INVOCATION_ID="a1b2c3d4-e5f6-7890-1234-567890abcdef"

curl http://localhost:8080/api/workflows/$INVOCATION_ID | jq
```

**Expected Response:**

You will see a JSON response with the details of the workflow execution. The `status` should be `completed`.

```json
{
  "invocation_id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
  "handler": "hello_workflow",
  "status": "completed",
  "current_step": -1,
  "current_step_type": "",
  "total_steps": 1,
  "journal": [
    {
      "step_number": 0,
      "step_type": "HelloService.SayHello",
      "error": ""
    }
  ]
}
```

You can also check the logs of the `processor` and `services` to see the workflow execution and the "Hello, REST Client!" message being processed.
