package main

import (
	"context"
	"fmt"
	"log"
	"time"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
	"github.com/sanjaymijar/my-durable-execution/pkg/durable"
)

// MultiServiceWorkflow demonstrates calling the same service multiple times
// Each call is journaled separately, making the workflow durable and resumable
var MultiServiceWorkflow = durable.NewWorkflow("multi_service_demo",
	func(ctx *durable.Context, topic string) (string, error) {
		log.Printf("🚀 Starting multi-service workflow for topic: %s", topic)

		// Step 1: Get greeting in English
		log.Println("📞 Call 1: Requesting English greeting...")
		req1 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (English)", topic)}
		resp1 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req1, resp1); err != nil {
			return "", fmt.Errorf("English greeting failed: %w", err)
		}
		log.Printf("✅ Call 1 completed: %s", resp1.Message)

		// Step 2: Get greeting in Spanish style
		log.Println("📞 Call 2: Requesting Spanish greeting...")
		req2 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (Spanish)", topic)}
		resp2 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req2, resp2); err != nil {
			return "", fmt.Errorf("Spanish greeting failed: %w", err)
		}
		log.Printf("✅ Call 2 completed: %s", resp2.Message)

		// Step 3: Get greeting in French style
		log.Println("📞 Call 3: Requesting French greeting...")
		req3 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (French)", topic)}
		resp3 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req3, resp3); err != nil {
			return "", fmt.Errorf("French greeting failed: %w", err)
		}
		log.Printf("✅ Call 3 completed: %s", resp3.Message)

		// Step 4: Get summary greeting
		log.Println("📞 Call 4: Requesting summary...")
		req4 := &hellopb.HelloRequest{Name: "Summary"}
		resp4 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req4, resp4); err != nil {
			return "", fmt.Errorf("Summary greeting failed: %w", err)
		}
		log.Printf("✅ Call 4 completed: %s", resp4.Message)

		// Combine all responses into a summary
		summary := fmt.Sprintf(
			"Multi-Service Workflow Results:\n"+
				"1️⃣  %s\n"+
				"2️⃣  %s\n"+
				"3️⃣  %s\n"+
				"📊 %s\n"+
				"Total service calls: 4",
			resp1.Message, resp2.Message, resp3.Message, resp4.Message,
		)

		log.Printf("🎉 Workflow completed successfully!")
		return summary, nil
	})

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Connect to NATS
	c, err := client.NewClient("nats://127.0.0.1:4322")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	fmt.Println("========================================")
	fmt.Println("Multi-Service Invocation Demo")
	fmt.Println("========================================")
	fmt.Println()

	// Example 1: Remote execution via NATS
	fmt.Println("Example 1: Remote Execution")
	fmt.Println("----------------------------")
	result, err := client.InvokeWorkflow[string, string](c, ctx, MultiServiceWorkflow, "Durable Execution")
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}
	fmt.Println(result)
	fmt.Println()

	// Example 2: Local execution (faster for development)
	fmt.Println("Example 2: Local Execution")
	fmt.Println("----------------------------")

	// For local execution, we need to provide the service invokers
	localServiceRegistry := durable.NewServiceRegistry()
	localServiceRegistry.Register("HelloService", durable.NewGRPCInvoker("127.0.0.1:9090"))
	defer localServiceRegistry.Close()

	// Set the service registry on the client for local execution
	c.SetServiceRegistry(localServiceRegistry)

	c.Register(MultiServiceWorkflow)
	go c.ServeHandlers(ctx)

	// Small delay to ensure server is ready
	time.Sleep(100 * time.Millisecond)

	localResult, err := client.InvokeWorkflow[string, string](c, ctx, MultiServiceWorkflow, "Local Development")
	if err != nil {
		log.Fatalf("Local workflow failed: %v", err)
	}
	fmt.Println(localResult)
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("✓ All examples completed!")
	fmt.Println("========================================")
}
