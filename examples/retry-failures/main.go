package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
	"github.com/sanjaymijar/my-durable-execution/pkg/durable"
)

// FlakyWorkflow simulates a workflow that might fail on first try
// It fails if the input contains "flaky", but succeeds if it contains "retry"
var FlakyWorkflow = durable.NewWorkflow("flaky_workflow",
	func(ctx *durable.Context, input string) (string, error) {
		log.Printf("🔧 Executing FlakyWorkflow for: %s", input)

		// Simulate transient failure on first attempt
		if strings.Contains(input, "flaky") && !strings.Contains(input, "retry") {
			log.Printf("❌ Simulating transient failure for: %s", input)
			return "", fmt.Errorf("transient error: service temporarily unavailable for %s", input)
		}

		// Step 1: Call service
		req := &hellopb.HelloRequest{Name: input}
		resp := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		log.Printf("✅ FlakyWorkflow succeeded for: %s", input)
		return resp.Message, nil
	})

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("========================================")
	fmt.Println("Workflow Failure Detection & Retry Demo")
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
	// PHASE 1: Submit workflows (some will fail)
	// ============================================================
	fmt.Println("📤 PHASE 1: Submitting workflows...")
	fmt.Println("-----------------------------------")

	inputs := []string{
		"task-success-1",     // Will succeed
		"task-flaky-1",       // Will fail (transient error)
		"task-success-2",     // Will succeed
		"task-flaky-2",       // Will fail (transient error)
	}

	type workflowInvocation struct {
		invocationID string
		input        string
	}

	invocations := make([]workflowInvocation, 0, len(inputs))

	for _, input := range inputs {
		// JSON-encode the input for the workflow
		inputBytes, err := json.Marshal(input)
		if err != nil {
			log.Printf("❌ Failed to marshal input %s: %v", input, err)
			continue
		}

		invocationID, err := c.InvokeAsync(ctx, "flaky_workflow", client.WithArgs(inputBytes))
		if err != nil {
			log.Printf("❌ Failed to submit %s: %v", input, err)
			continue
		}
		invocations = append(invocations, workflowInvocation{invocationID, input})
		fmt.Printf("  ✓ Submitted: %s (invocation_id: %s)\n", input, invocationID[:8]+"...")
	}

	// Wait for all to complete
	fmt.Println("\n⏳ Waiting for workflows to complete...")
	time.Sleep(2 * time.Second)

	// ============================================================
	// PHASE 2: Check status and identify failures
	// ============================================================
	fmt.Println("\n📊 PHASE 2: Checking workflow status...")
	fmt.Println("---------------------------------------")

	successfulWorkflows := make([]workflowInvocation, 0)
	failedWorkflows := make([]workflowInvocation, 0)

	for _, inv := range invocations {
		result, err := c.GetResult(ctx, inv.invocationID)
		if err != nil {
			log.Printf("⚠️  Could not get result for %s: %v", inv.input, err)
			continue
		}

		fmt.Printf("\n  Workflow: %s\n", inv.input)
		fmt.Printf("    Invocation ID: %s\n", inv.invocationID[:16]+"...")
		fmt.Printf("    Status: %s\n", result.Status)

		if result.Status == "failed" {
			fmt.Printf("    ❌ FAILED\n")
			failedWorkflows = append(failedWorkflows, inv)
		} else if result.Status == "completed" {
			fmt.Printf("    ✅ SUCCESS\n")
			successfulWorkflows = append(successfulWorkflows, inv)
		}
	}

	// ============================================================
	// PHASE 3: Summary
	// ============================================================
	fmt.Println("\n📋 Summary:")
	fmt.Printf("  Total submitted: %d\n", len(invocations))
	fmt.Printf("  ✅ Successful: %d\n", len(successfulWorkflows))
	fmt.Printf("  ❌ Failed: %d\n", len(failedWorkflows))

	if len(failedWorkflows) == 0 {
		fmt.Println("\n🎉 All workflows succeeded! Nothing to retry.")
		return
	}

	// ============================================================
	// PHASE 4: Retry only failed workflows
	// ============================================================
	fmt.Println("\n🔄 PHASE 4: Retrying failed workflows...")
	fmt.Println("----------------------------------------")

	retriedInvocations := make([]workflowInvocation, 0)

	for _, failed := range failedWorkflows {
		// Modify input to include "retry" so it succeeds this time
		retryInput := failed.input + "-retry"
		fmt.Printf("\n  Retrying: %s → %s\n", failed.input, retryInput)

		// JSON-encode the retry input
		retryBytes, err := json.Marshal(retryInput)
		if err != nil {
			log.Printf("    ❌ Failed to marshal retry input: %v", err)
			continue
		}

		invocationID, err := c.InvokeAsync(ctx, "flaky_workflow", client.WithArgs(retryBytes))
		if err != nil {
			log.Printf("    ❌ Failed to retry: %v", err)
			continue
		}

		retriedInvocations = append(retriedInvocations, workflowInvocation{invocationID, retryInput})
		fmt.Printf("    ✓ Resubmitted (invocation_id: %s)\n", invocationID[:8]+"...")
	}

	// Wait for retries to complete
	fmt.Println("\n⏳ Waiting for retries to complete...")
	time.Sleep(2 * time.Second)

	// ============================================================
	// PHASE 5: Verify retry results
	// ============================================================
	fmt.Println("\n✅ PHASE 5: Verifying retry results...")
	fmt.Println("--------------------------------------")

	retrySuccessCount := 0
	retryFailCount := 0

	for _, inv := range retriedInvocations {
		result, err := c.GetResult(ctx, inv.invocationID)
		if err != nil {
			log.Printf("  ⚠️  Could not get result for %s: %v", inv.input, err)
			retryFailCount++
			continue
		}

		fmt.Printf("\n  Retry: %s\n", inv.input)
		fmt.Printf("    Status: %s\n", result.Status)

		if result.Status == "completed" {
			fmt.Printf("    ✅ RETRY SUCCEEDED!\n")
			retrySuccessCount++
		} else {
			fmt.Printf("    ❌ Retry still failed\n")
			retryFailCount++
		}
	}

	// ============================================================
	// FINAL SUMMARY
	// ============================================================
	fmt.Println("\n" + strings.Repeat("=", 40))
	fmt.Println("🎯 FINAL RESULTS")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("\nOriginal Run:\n")
	fmt.Printf("  ✅ Successful: %d/%d\n", len(successfulWorkflows), len(invocations))
	fmt.Printf("  ❌ Failed: %d/%d\n", len(failedWorkflows), len(invocations))
	fmt.Printf("\nRetry Run:\n")
	fmt.Printf("  ✅ Successful: %d/%d\n", retrySuccessCount, len(retriedInvocations))
	fmt.Printf("  ❌ Failed: %d/%d\n", retryFailCount, len(retriedInvocations))
	fmt.Printf("\nOverall Success Rate:\n")
	totalSuccess := len(successfulWorkflows) + retrySuccessCount
	totalAttempts := len(invocations)
	fmt.Printf("  %d/%d (%.1f%%) workflows succeeded\n",
		totalSuccess, totalAttempts, float64(totalSuccess)/float64(totalAttempts)*100)

	fmt.Println("\n" + strings.Repeat("=", 40))
	fmt.Println("✓ Demo completed!")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Println()
	fmt.Println("💡 Key Takeaways:")
	fmt.Println("  • Failed workflows are identified by Status == \"failed\"")
	fmt.Println("  • You can collect failed invocation IDs in an array")
	fmt.Println("  • Retry by resubmitting with the same or modified input")
	fmt.Println("  • Each retry gets a new invocation ID")
	fmt.Println("  • Journal ensures completed steps aren't re-executed")
}
