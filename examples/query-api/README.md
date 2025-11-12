# Query API Example

This example demonstrates how to use the client's query API to inspect the state of workflows. It also highlights the equivalent HTTP endpoints that are available when the processor is running its API server.

## How to Run

Ensure the NATS cluster, processor, and services are running first. The processor must have its HTTP API enabled (which it is by default).

```bash
# In separate terminals
make docker-up
make run-service
make run-processor
```

Then, run the example from the **root directory** of the project:

```bash
make run-query
```

## Expected Output

The example submits several workflows and then demonstrates various ways to query them, both programmatically and via `curl`.

```
Running query API example...
./bin/query-api
========================================
Query API Demo
========================================

📤 Phase 1: Submitting 5 workflows...
...

🔍 Phase 2: Querying specific workflow...
...

📋 Phase 3: Listing all workflows...
...

🔎 Phase 4: Filtering workflows by status...
...

📊 Phase 5: Getting workflow statistics...
...

========================================
HTTP API Examples (curl)
========================================

Try these curl commands in another terminal:

# Get specific workflow:
curl http://localhost:8080/api/workflows/{invocation_id} | jq

# List all workflows:
curl http://localhost:8080/api/workflows | jq
...

========================================
✓ Query API Demo Complete!
========================================
```

## Key Features Demonstrated

*   **Get Workflow:** Retrieving the detailed state of a single workflow using `client.GetWorkflow`.
*   **List Workflows:** Listing all workflows with optional filtering using `client.ListWorkflows`.
*   **Workflow Stats:** Getting aggregate statistics about all workflows using `client.GetWorkflowStats`.
*   **HTTP API:** Shows the corresponding `curl` commands for interacting with the processor's built-in REST API for workflow introspection.
