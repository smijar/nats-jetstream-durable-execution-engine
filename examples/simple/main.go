package main

import (
	"context"
	"fmt"
	"log"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
)

func main() {
	// Create client and invoke workflow
	c, _ := client.NewClient("nats://127.0.0.1:4322")
	defer c.Close()

	result, _ := c.Invoke(context.Background(), "hello_workflow")

	// Extract response
	var resp hellopb.HelloResponse
	result.GetFirstResponse(&resp)

	fmt.Println(resp.Message)
}

// Even simpler with error handling:
func simpleExample() {
	c, err := client.NewClient("nats://127.0.0.1:4322")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	result, err := c.Invoke(context.Background(), "hello_workflow")
	if err != nil {
		log.Fatal(err)
	}

	var resp hellopb.HelloResponse
	if err := result.GetFirstResponse(&resp); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Response:", resp.Message)
}
