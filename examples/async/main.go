package main

import (
	"context"
	"fmt"
	"log"
	"time"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
)

func main() {
	c, err := client.NewClient("nats://127.0.0.1:4322")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()

	// Fire and forget - submit without waiting
	invocationID, err := c.InvokeAsync(ctx, "hello_workflow")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Submitted workflow: %s\n", invocationID)

	// Do other work...
	fmt.Println("Doing other work while workflow executes...")
	time.Sleep(2 * time.Second)

	// Check the result later
	result, err := c.GetResult(ctx, invocationID)
	if err != nil {
		log.Fatal(err)
	}

	var resp hellopb.HelloResponse
	if err := result.GetFirstResponse(&resp); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Result: %s (status: %s)\n", resp.Message, result.Status)
}
