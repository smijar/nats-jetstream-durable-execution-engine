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

// ====================================================================================
// STEP 1: Define Workflows (like Restate services)
// ====================================================================================

// HelloWorkflow - simple string in/out
var HelloWorkflow = durable.NewWorkflow("greeting",
	func(ctx *durable.Context, name string) (string, error) {
		req := &hellopb.HelloRequest{Name: name}
		resp := &hellopb.HelloResponse{}

		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		return resp.Message, nil
	})

// GreetManyWorkflow - composition example
var GreetManyWorkflow = durable.NewWorkflow("greet_many",
	func(ctx *durable.Context, names []string) ([]string, error) {
		results := make([]string, 0, len(names))

		for _, name := range names {
			req := &hellopb.HelloRequest{Name: name}
			resp := &hellopb.HelloResponse{}

			if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
				return nil, err
			}

			results = append(results, resp.Message)
		}

		return results, nil
	})

func main() {
	fmt.Println("========================================")
	fmt.Println("Restate-Style Durable Execution")
	fmt.Println("========================================")
	fmt.Println()

	// Create client
	c, err := client.NewClient("nats://127.0.0.1:4322")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()

	// ====================================================================================
	// Example 1: Type-Safe Remote Invocation (like Restate ctx.run)
	// ====================================================================================

	fmt.Println("Example 1: Type-Safe Remote Invocation")
	fmt.Println("---------------------------------------")

	// Invoke with full type safety!
	greeting, err := client.InvokeWorkflow[string, string](c, ctx, HelloWorkflow, "Restate-Style")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Input:  string = \"Restate-Style\"\n")
	fmt.Printf("Output: string = \"%s\"\n", greeting)
	fmt.Println()

	// ====================================================================================
	// Example 2: Invoke with Reference (no handler code needed on client)
	// ====================================================================================

	fmt.Println("Example 2: Remote Workflow Reference")
	fmt.Println("---------------------------------------")

	// Create a reference - client doesn't need handler implementation
	workflowRef := HelloWorkflow.Ref()

	greeting2, err := client.InvokeWorkflow[string, string](c, ctx, workflowRef, "Remote")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Result: %s\n", greeting2)
	fmt.Println()

	// ====================================================================================
	// Example 3: Local Development Mode (everything in one process)
	// ====================================================================================

	fmt.Println("Example 3: Local Development Mode")
	fmt.Println("---------------------------------------")

	// Register for local execution
	c.Register(HelloWorkflow)
	c.Register(GreetManyWorkflow)

	// Start serving locally
	localCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.ServeHandlers(localCtx)

	// Give ServeHandlers time to start
	time.Sleep(100 * time.Millisecond)

	// Now invocations run locally without NATS!
	localResult, err := client.InvokeWorkflow[string, string](c, localCtx, HelloWorkflow, "Local")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Local result: %s\n", localResult)
	fmt.Println()

	// ====================================================================================
	// Example 4: Workflow Composition (workflow calling workflow)
	// ====================================================================================

	fmt.Println("Example 4: Workflow Composition")
	fmt.Println("---------------------------------------")

	names := []string{"Alice", "Bob", "Charlie"}
	greetings, err := client.InvokeWorkflow[[]string, []string](c, localCtx, GreetManyWorkflow, names)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Input:  []string = %v\n", names)
	fmt.Printf("Output: []string = \n")
	for i, g := range greetings {
		fmt.Printf("  [%d] %s\n", i, g)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("✓ All examples completed!")
	fmt.Println("========================================")
}
