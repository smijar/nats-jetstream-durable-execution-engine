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
	ctx := context.Background()

	c, err := client.NewClient("nats://127.0.0.1:4322")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// For local execution, we need to provide the service invokers
	localServiceRegistry := durable.NewServiceRegistry()
	localServiceRegistry.Register("HelloService", durable.NewGRPCInvoker("127.0.0.1:9090"))
	defer localServiceRegistry.Close()

	// Set the service registry on the client for local execution
	c.SetServiceRegistry(localServiceRegistry)

	// Register workflow handler locally
	c.Register(HelloWorkflow)
	go func() {
		if err := c.ServeHandlers(ctx); err != nil {
			log.Printf("Local handler server error: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond) // Give server time to start

	// Now invoke the workflow - it runs locally, no NATS needed!
	result, err := client.InvokeWorkflow[string, string](c, ctx, HelloWorkflow, "Local World")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✓ Executed locally!")
	fmt.Println("Result:", result)
}
