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

// SlowWorkflow demonstrates a multi-step workflow that can be paused/cancelled
var SlowWorkflow = durable.NewWorkflow("slow_workflow",
	func(ctx *durable.Context, taskName string) (string, error) {
		log.Printf("🚀 Starting SlowWorkflow: %s", taskName)
		results := []string{}

		// Step 1
		log.Printf("📞 [%s] Step 1/3: Initial call...", taskName)
		req1 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s-step1", taskName)}
		resp1 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req1, resp1); err != nil {
			return "", err
		}
		results = append(results, resp1.Message)
		log.Printf("✅ [%s] Step 1 completed", taskName)

		// Step 2
		log.Printf("📞 [%s] Step 2/3: Middle call...", taskName)
		req2 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s-step2", taskName)}
		resp2 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req2, resp2); err != nil {
			return "", err
		}
		results = append(results, resp2.Message)
		log.Printf("✅ [%s] Step 2 completed", taskName)

		// Step 3
		log.Printf("📞 [%s] Step 3/3: Final call...", taskName)
		req3 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s-step3", taskName)}
		resp3 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req3, resp3); err != nil {
			return "", err
		}
		results = append(results, resp3.Message)
		log.Printf("✅ [%s] Step 3 completed", taskName)

		result := fmt.Sprintf("SlowWorkflow[%s] completed all 3 steps:\n  %s",
			taskName, strings.Join(results, "\n  "))
		log.Printf("🎉 [%s] Workflow completed!", taskName)
		return result, nil
	})

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Workflow Pause/Resume & Cancel Demo")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// Connect to NATS
	c, err := client.NewClientWithTimeout("nats://127.0.0.1:4322", 60*time.Second)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// ============================================================
	// DEMO 1: Pause and Resume
	// ============================================================
	fmt.Println("📋 DEMO 1: Pause & Resume")
	fmt.Println(strings.Repeat("-", 60))

	// Submit workflow
	inputBytes, _ := json.Marshal("pausable-task")
	invocationID1, err := c.InvokeAsync(ctx, "slow_workflow", client.WithArgs(inputBytes))
	if err != nil {
		log.Fatalf("Failed to submit workflow: %v", err)
	}
	fmt.Printf("✓ Submitted workflow (invocation_id: %s)\n\n", invocationID1[:16]+"...")

	// Let it run for a bit (complete step 1)
	fmt.Println("⏳ Letting workflow run for 1 second...")
	time.Sleep(1 * time.Second)

	// Pause the workflow
	fmt.Println("\n⏸️  PAUSING workflow...")
	if err := c.Pause(ctx, invocationID1); err != nil {
		log.Printf("Warning: Failed to pause: %v", err)
	} else {
		fmt.Println("✓ Workflow paused")
	}

	// Check status
	time.Sleep(500 * time.Millisecond)
	result1, _ := c.GetResult(ctx, invocationID1)
	fmt.Printf("\nStatus check: %s\n", result1.Status)
	fmt.Printf("Journal entries: %d\n", len(result1.Journal))

	// Wait while paused
	fmt.Println("\n💤 Workflow is paused... waiting 2 seconds...")
	time.Sleep(2 * time.Second)

	// Resume the workflow
	fmt.Println("\n▶️  RESUMING workflow...")
	if err := c.Resume(ctx, invocationID1); err != nil {
		log.Printf("Warning: Failed to resume: %v", err)
	} else {
		fmt.Println("✓ Workflow resumed")
	}

	// Wait for completion
	fmt.Println("\n⏳ Waiting for workflow to complete...")
	time.Sleep(3 * time.Second)

	// Check final result
	result1, err = c.GetResult(ctx, invocationID1)
	if err != nil {
		log.Printf("Failed to get result: %v", err)
	} else {
		fmt.Printf("\n✅ Final Status: %s\n", result1.Status)
		fmt.Printf("Journal entries: %d\n", len(result1.Journal))
		if result1.Status == "completed" {
			var output string
			json.Unmarshal(result1.CurrentState, &output)
			fmt.Printf("\nOutput:\n%s\n", output)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))

	// ============================================================
	// DEMO 2: Cancel
	// ============================================================
	fmt.Println("\n📋 DEMO 2: Cancel Workflow")
	fmt.Println(strings.Repeat("-", 60))

	// Submit workflow
	inputBytes2, _ := json.Marshal("cancellable-task")
	invocationID2, err := c.InvokeAsync(ctx, "slow_workflow", client.WithArgs(inputBytes2))
	if err != nil {
		log.Fatalf("Failed to submit workflow: %v", err)
	}
	fmt.Printf("✓ Submitted workflow (invocation_id: %s)\n\n", invocationID2[:16]+"...")

	// Let it run for a bit
	fmt.Println("⏳ Letting workflow run for 1 second...")
	time.Sleep(1 * time.Second)

	// Cancel the workflow
	fmt.Println("\n🛑 CANCELLING workflow...")
	if err := c.Cancel(ctx, invocationID2); err != nil {
		log.Printf("Warning: Failed to cancel: %v", err)
	} else {
		fmt.Println("✓ Workflow cancelled")
	}

	// Wait a bit
	time.Sleep(1 * time.Second)

	// Check status
	result2, _ := c.GetResult(ctx, invocationID2)
	fmt.Printf("\n✅ Final Status: %s\n", result2.Status)
	fmt.Printf("Journal entries completed before cancellation: %d\n", len(result2.Journal))

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("✓ Demo completed!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	fmt.Println("💡 Key Differences:")
	fmt.Println("  PAUSE/RESUME:")
	fmt.Println("    • Same invocation ID continues")
	fmt.Println("    • Resumes from journal (where it left off)")
	fmt.Println("    • Use for: temporary holds, rate limiting, waiting for resources")
	fmt.Println()
	fmt.Println("  CANCEL + RETRY:")
	fmt.Println("    • Cancel stops execution permanently")
	fmt.Println("    • Retry creates NEW invocation ID")
	fmt.Println("    • Retry starts fresh (may use journal for idempotency)")
	fmt.Println("    • Use for: permanent cancellation, retry with different input")
}
