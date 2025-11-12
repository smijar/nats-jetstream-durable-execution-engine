package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
	"github.com/sanjaymijar/my-durable-execution/pkg/durable"
)

// ====================================================================================
// Delayed Execution Example
// ====================================================================================

// SchedulerInput represents input for the scheduler workflow
type SchedulerInput struct {
	Message string `json:"message"`
	Delay   int    `json:"delay_seconds"`
}

// GreetingInput represents input for greeting workflow
type GreetingInput struct {
	Name string `json:"name"`
}

// SchedulerWorkflow - Demonstrates delayed workflow invocation
var SchedulerWorkflow = durable.NewWorkflow("scheduler",
	func(ctx *durable.Context, input SchedulerInput) (string, error) {
		log.Printf("📅 Scheduler workflow started: message=%s, delay=%ds", input.Message, input.Delay)

		// Call HelloService first
		req := &hellopb.HelloRequest{Name: "Scheduler"}
		resp := &hellopb.HelloResponse{}

		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		log.Printf("📞 HelloService response: %s", resp.Message)

		// Schedule a delayed greeting
		delayDuration := time.Duration(input.Delay) * time.Second
		greetingInput := GreetingInput{Name: input.Message}

		if err := ctx.SendDelayed("delayed_greeting", greetingInput, delayDuration, "delayed-greeting-partition"); err != nil {
			log.Printf("❌ Failed to schedule delayed greeting: %v", err)
			return "", err
		}

		result := fmt.Sprintf("Scheduled greeting for '%s' to execute in %d seconds", input.Message, input.Delay)
		log.Printf("✅ %s", result)

		return result, nil
	})

// DelayedGreetingWorkflow - Gets invoked after delay
var DelayedGreetingWorkflow = durable.NewWorkflow("delayed_greeting",
	func(ctx *durable.Context, input GreetingInput) (string, error) {
		log.Printf("🎉 Delayed greeting workflow executing for: %s", input.Name)

		req := &hellopb.HelloRequest{Name: input.Name}
		resp := &hellopb.HelloResponse{}

		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		result := fmt.Sprintf("✅ Delayed execution completed! Message: %s", resp.Message)
		log.Printf("%s", result)

		return result, nil
	})

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("========================================")
	fmt.Println("Delayed Execution Example")
	fmt.Println("========================================")
	fmt.Println()

	// Connect to NATS
	c, err := client.NewClient("nats://127.0.0.1:4322")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// ==============================================================
	// Scenario: Schedule a workflow to run after 30 seconds
	// ==============================================================
	fmt.Println("📤 Scheduling a delayed greeting (30 seconds)...")
	fmt.Println()

	input := SchedulerInput{
		Message: "Delayed Hello World",
		Delay:   30,
	}

	result, err := client.InvokeWorkflow[SchedulerInput, string](c, ctx, SchedulerWorkflow, input)
	if err != nil {
		log.Fatalf("Failed to invoke scheduler workflow: %v", err)
	}

	fmt.Printf("✅ Result: %s\n", result)
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("✓ Scheduler workflow completed!")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("The delayed greeting workflow will execute in 30 seconds.")
	fmt.Println("Check the processor logs to see when it runs!")
	fmt.Println()
	fmt.Println("Key Features Demonstrated:")
	fmt.Println("  ✅ Delayed workflow invocation (ctx.SendDelayed)")
	fmt.Println("  ✅ Durable RPC calls (ctx.DurableCall)")
	fmt.Println("  ✅ Message scheduling with NATS headers")
	fmt.Println("  ✅ Automatic requeue for future execution")

	// Suppress unused variable warning
	_ = json.Marshal
}
