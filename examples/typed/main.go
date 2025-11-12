package main

import (
	"context"
	"fmt"
	"log"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
	"github.com/sanjaymijar/my-durable-execution/pkg/durable"
)

// HelloWorkflowInput represents input for HelloWorkflow
type HelloWorkflowInput struct {
	Name string `json:"name"`
}

// Define workflows with full type safety
var HelloWorkflow = durable.NewWorkflow("hello_workflow",
	func(ctx *durable.Context, input HelloWorkflowInput) (string, error) {
		req := &hellopb.HelloRequest{Name: input.Name}
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

	// Type-safe invocation - input is HelloWorkflowInput, output is string
	input := HelloWorkflowInput{Name: "TypeSafe World"}
	result, err := client.InvokeWorkflow[HelloWorkflowInput, string](c, context.Background(), HelloWorkflow, input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Result:", result)
	// Result will be fully typed as string!
}
