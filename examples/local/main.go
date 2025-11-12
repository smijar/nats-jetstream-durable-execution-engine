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

// Define workflow
var HelloWorkflow = durable.NewWorkflow("hello_local",
	func(ctx *durable.Context, name string) (string, error) {
		req := &hellopb.HelloRequest{Name: name}
		resp := &hellopb.HelloResponse{}

		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		return resp.Message, nil
	})

func main() {
	c, err := client.NewClient("nats://127.0.0.1:4322")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Register workflow for local execution
	c.Register(HelloWorkflow)

	// Serve handlers locally in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := c.ServeHandlers(ctx); err != nil && err != context.Canceled {
			log.Printf("Server error: %v", err)
		}
	}()

	// Give ServeHandlers time to start
	time.Sleep(100 * time.Millisecond)

	// Now invoke the workflow - it runs locally, no NATS needed!
	result, err := client.InvokeWorkflow[string, string](c, ctx, HelloWorkflow, "Local World")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✓ Executed locally!")
	fmt.Println("Result:", result)
}
