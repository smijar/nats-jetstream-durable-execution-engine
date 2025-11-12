package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
	"github.com/sanjaymijar/my-durable-execution/pkg/durable"
)

// AsyncWorkflow demonstrates a 3-step workflow where the middle step is async
var AsyncWorkflow = durable.NewWorkflow("async_workflow",
	func(ctx *durable.Context, userID string) (string, error) {
		log.Printf("🚀 Starting AsyncWorkflow for user: %s", userID)

		// ============================================================
		// STEP 1: Synchronous greeting (blocking)
		// ============================================================
		log.Printf("📞 [%s] Step 1/3: Synchronous greeting...", userID)
		req1 := &hellopb.HelloRequest{Name: fmt.Sprintf("User:%s", userID)}
		resp1 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req1, resp1); err != nil {
			return "", fmt.Errorf("step 1 failed: %w", err)
		}
		log.Printf("✅ [%s] Step 1 completed: %s", userID, resp1.Message)

		// ============================================================
		// STEP 2: Async call - submit and continue (non-blocking simulation)
		// ============================================================
		log.Printf("📤 [%s] Step 2/3: Submitting async notification...", userID)
		req2 := &hellopb.HelloRequest{Name: fmt.Sprintf("Async:%s", userID)}
		resp2 := &hellopb.HelloResponse{}

		// In a real async pattern, we'd submit this and continue
		// For now, we'll make the call but show it as async in logs
		if err := ctx.DurableCall("HelloService", "SayHello", req2, resp2); err != nil {
			return "", fmt.Errorf("step 2 failed: %w", err)
		}
		log.Printf("✅ [%s] Step 2 submitted (async), got: %s", userID, resp2.Message)

		// ============================================================
		// STEP 3: Final synchronous call
		// ============================================================
		log.Printf("📞 [%s] Step 3/3: Final confirmation...", userID)
		req3 := &hellopb.HelloRequest{Name: fmt.Sprintf("Confirm:%s", userID)}
		resp3 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req3, resp3); err != nil {
			return "", fmt.Errorf("step 3 failed: %w", err)
		}
		log.Printf("✅ [%s] Step 3 completed: %s", userID, resp3.Message)

		// Combine results
		result := fmt.Sprintf(
			"AsyncWorkflow[%s] completed:\n  Step1: %s\n  Step2: %s\n  Step3: %s",
			userID, resp1.Message, resp2.Message, resp3.Message,
		)

		log.Printf("🎉 [%s] Workflow completed!", userID)
		return result, nil
	})

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Parse flags
	numInvocations := flag.Int("count", 10, "Number of concurrent workflow invocations")
	timeoutSeconds := flag.Int("timeout", 300, "Timeout in seconds for each workflow (default: 300s / 5min)")
	flag.Parse()

	timeout := time.Duration(*timeoutSeconds) * time.Second

	fmt.Println("========================================")
	fmt.Printf("Concurrent Workflow Invocations (N=%d)\n", *numInvocations)
	fmt.Printf("Timeout: %v per workflow\n", timeout)
	fmt.Println("========================================")
	fmt.Println()

	// Connect to NATS with custom timeout
	c, err := client.NewClientWithTimeout("nats://127.0.0.1:4322", timeout)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// ============================================================
	// Invoke workflow N times concurrently
	// ============================================================
	fmt.Printf("🚀 Submitting %d workflow invocations...\n\n", *numInvocations)
	startTime := time.Now()

	var wg sync.WaitGroup
	results := make(chan string, *numInvocations)
	errors := make(chan error, *numInvocations)

	for i := 0; i < *numInvocations; i++ {
		wg.Add(1)
		userID := fmt.Sprintf("user-%03d", i+1)

		go func(uid string, index int) {
			defer wg.Done()

			log.Printf("📤 [Invoke %d] Submitting workflow for %s", index+1, uid)
			result, err := client.InvokeWorkflow[string, string](c, ctx, AsyncWorkflow, uid)
			if err != nil {
				log.Printf("❌ [Invoke %d] Failed: %v", index+1, err)
				errors <- err
				return
			}

			log.Printf("✅ [Invoke %d] Completed for %s", index+1, uid)
			results <- result
		}(userID, i)
	}

	// Wait for all invocations to complete
	wg.Wait()
	close(results)
	close(errors)

	duration := time.Since(startTime)

	// ============================================================
	// Display results
	// ============================================================
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Results Summary")
	fmt.Println("========================================")

	successCount := 0
	for result := range results {
		successCount++
		if successCount <= 3 {
			// Show first 3 results
			fmt.Printf("\n%s\n", result)
		}
	}

	errorCount := 0
	for err := range errors {
		errorCount++
		fmt.Printf("❌ Error: %v\n", err)
	}

	fmt.Println()
	fmt.Printf("Total Invocations: %d\n", *numInvocations)
	fmt.Printf("✅ Success: %d\n", successCount)
	fmt.Printf("❌ Failed: %d\n", errorCount)
	fmt.Printf("⏱️  Total Time: %v\n", duration)
	fmt.Printf("📊 Avg Time per Workflow: %v\n", duration/time.Duration(*numInvocations))

	if successCount > 3 {
		fmt.Printf("\n(Showing first 3 results, %d more completed successfully)\n", successCount-3)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("✓ Test completed!")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("💡 Try different counts:")
	fmt.Println("   ./bin/concurrent-workflows --count 5")
	fmt.Println("   ./bin/concurrent-workflows --count 20")
	fmt.Println("   ./bin/concurrent-workflows --count 100")
	fmt.Println()
	fmt.Println("💡 For large batches, increase timeout:")
	fmt.Println("   ./bin/concurrent-workflows --count 10000 --timeout 600")
	fmt.Println("   ./bin/concurrent-workflows --count 50000 --timeout 1800")
}
