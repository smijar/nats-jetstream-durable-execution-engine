package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sanjaymijar/my-durable-execution/pkg/client"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("========================================")
	fmt.Println("Query API Demo")
	fmt.Println("========================================")
	fmt.Println()

	// Connect to NATS
	c, err := client.NewClient("nats://127.0.0.1:4322")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// ============================================================
	// Phase 1: Submit some workflows
	// ============================================================
	fmt.Println("📤 Phase 1: Submitting 5 workflows...")
	fmt.Println()

	invocations := []string{}

	for i := 0; i < 5; i++ {
		input := fmt.Sprintf("user-%d", i+1)
		inputBytes, _ := json.Marshal(input)

		invocationID, err := c.InvokeAsync(ctx, "async_workflow", client.WithArgs(inputBytes))
		if err != nil {
			log.Printf("Failed to submit workflow %d: %v", i+1, err)
			continue
		}

		invocations = append(invocations, invocationID)
		fmt.Printf("  ✅ Submitted workflow %d: %s\n", i+1, invocationID)
	}

	fmt.Println()
	fmt.Println("⏳ Waiting 2 seconds for workflows to start...")
	time.Sleep(2 * time.Second)

	// ============================================================
	// Phase 2: Query specific workflow
	// ============================================================
	fmt.Println()
	fmt.Println("🔍 Phase 2: Querying specific workflow...")
	fmt.Println()

	if len(invocations) > 0 {
		details, err := c.GetWorkflow(ctx, invocations[0])
		if err != nil {
			log.Printf("Failed to get workflow: %v", err)
		} else {
			fmt.Printf("Workflow Details:\n")
			fmt.Printf("  Invocation ID: %s\n", details.InvocationID)
			fmt.Printf("  Handler: %s\n", details.Handler)
			fmt.Printf("  Status: %s\n", details.Status)
			fmt.Printf("  Current Step: %d\n", details.CurrentStep)
			fmt.Printf("  Current Step Type: %s\n", details.CurrentStepType)
			fmt.Printf("  Total Steps: %d\n", details.TotalSteps)
			fmt.Printf("  Journal Entries: %d\n", len(details.Journal))
			fmt.Println()
		}
	}

	// ============================================================
	// Phase 3: List all workflows
	// ============================================================
	fmt.Println("📋 Phase 3: Listing all workflows...")
	fmt.Println()

	list, err := c.ListWorkflows(ctx)
	if err != nil {
		log.Printf("Failed to list workflows: %v", err)
	} else {
		fmt.Printf("Total workflows: %d\n", list.Total)
		fmt.Println()
		for i, wf := range list.Workflows {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", list.Total-10)
				break
			}
			fmt.Printf("  %d. [%s] %s - %s (Step %d/%d)\n",
				i+1, wf.Status, wf.Handler, wf.InvocationID[:8], wf.CurrentStep+1, wf.TotalSteps)
		}
		fmt.Println()
	}

	// ============================================================
	// Phase 4: Filter by status
	// ============================================================
	fmt.Println("🔎 Phase 4: Filtering workflows by status...")
	fmt.Println()

	statuses := []string{"running", "completed", "failed", "paused"}
	for _, status := range statuses {
		filtered, err := c.ListWorkflows(ctx, client.WithStatus(status))
		if err != nil {
			continue
		}
		fmt.Printf("  %s: %d workflows\n", status, filtered.Total)
	}
	fmt.Println()

	// ============================================================
	// Phase 5: Get workflow statistics
	// ============================================================
	fmt.Println("📊 Phase 5: Getting workflow statistics...")
	fmt.Println()

	stats, err := c.GetWorkflowStats(ctx)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		fmt.Printf("Total Workflows: %d\n", stats.Total)
		fmt.Println()

		fmt.Println("By Status:")
		for status, count := range stats.ByStatus {
			fmt.Printf("  %s: %d\n", status, count)
		}
		fmt.Println()

		fmt.Println("By Handler:")
		for handler, count := range stats.ByHandler {
			fmt.Printf("  %s: %d\n", handler, count)
		}
		fmt.Println()
	}

	// ============================================================
	// Phase 6: HTTP API Examples
	// ============================================================
	fmt.Println("========================================")
	fmt.Println("HTTP API Examples (curl)")
	fmt.Println("========================================")
	fmt.Println()

	fmt.Println("Try these curl commands in another terminal:")
	fmt.Println()

	if len(invocations) > 0 {
		fmt.Printf("# Get specific workflow:\n")
		fmt.Printf("curl http://localhost:8080/api/workflows/%s | jq\n", invocations[0])
		fmt.Println()
	}

	fmt.Println("# List all workflows:")
	fmt.Println("curl http://localhost:8080/api/workflows | jq")
	fmt.Println()

	fmt.Println("# Filter by status:")
	fmt.Println("curl 'http://localhost:8080/api/workflows?status=running' | jq")
	fmt.Println("curl 'http://localhost:8080/api/workflows?status=completed' | jq")
	fmt.Println()

	fmt.Println("# Get workflow statistics:")
	fmt.Println("curl http://localhost:8080/api/workflows/stats | jq")
	fmt.Println()

	fmt.Println("# Limit results:")
	fmt.Println("curl 'http://localhost:8080/api/workflows?limit=5' | jq")
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("✓ Query API Demo Complete!")
	fmt.Println("========================================")
}
