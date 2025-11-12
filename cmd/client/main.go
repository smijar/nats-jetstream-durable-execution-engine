package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/sanjaymijar/my-durable-execution/pb/durable"
	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/execution"
	"github.com/sanjaymijar/my-durable-execution/pkg/jetstream"
	"google.golang.org/protobuf/proto"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Durable Execution Client...")

	// Connect to NATS
	natsURL := getEnv("NATS_URL", "nats://127.0.0.1:4322")
	jsClient, err := jetstream.NewClient(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer jsClient.Close()

	// Create processor (for command submission only)
	processor, err := execution.NewProcessor(jsClient)
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	ctx := context.Background()

	// Get handler name from args or use default
	handlerName := getEnv("HANDLER", "hello_workflow")
	partitionKey := getEnv("PARTITION_KEY", "default")

	// Submit a command
	cmd := &durable.Command{
		InvocationId: uuid.New().String(),
		Handler:      handlerName,
		Service:      "HelloService",
		Args:         []byte{},
		PartitionKey: partitionKey,
		Sequence:     1,
	}

	log.Printf("Submitting workflow:")
	log.Printf("  Invocation ID: %s", cmd.InvocationId)
	log.Printf("  Handler: %s", cmd.Handler)
	log.Printf("  Partition Key: %s", cmd.PartitionKey)

	if err := processor.SubmitCommand(ctx, cmd); err != nil {
		log.Fatalf("Failed to submit command: %v", err)
	}

	log.Println("✓ Command submitted successfully!")
	log.Println("Waiting for execution to complete...")

	// Wait for execution to complete and get the result
	state, err := waitForCompletion(ctx, jsClient, cmd.InvocationId, 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to get execution result: %v", err)
	}

	// Print the result
	printExecutionResult(state)
}

// waitForCompletion polls the KV bucket until execution completes or times out
func waitForCompletion(ctx context.Context, jsClient *jetstream.Client, invocationID string, timeout time.Duration) (*durable.ExecutionState, error) {
	kv, err := jsClient.GetStateKV(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket: %w", err)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for execution to complete")
			}

			entry, err := kv.Get(ctx, invocationID)
			if err != nil {
				if err == natsjs.ErrKeyNotFound {
					// State not yet created, keep waiting
					continue
				}
				return nil, fmt.Errorf("failed to get state: %w", err)
			}

			var state durable.ExecutionState
			if err := proto.Unmarshal(entry.Value(), &state); err != nil {
				return nil, fmt.Errorf("failed to unmarshal state: %w", err)
			}

			// Check if execution is complete
			if state.Status == "completed" || state.Status == "failed" {
				return &state, nil
			}
		}
	}
}

// printExecutionResult prints the execution result in a nice format
func printExecutionResult(state *durable.ExecutionState) {
	fmt.Println("")
	fmt.Println("========================================")
	fmt.Println("Execution Result")
	fmt.Println("========================================")
	fmt.Printf("Invocation ID: %s\n", state.InvocationId)
	fmt.Printf("Handler:       %s\n", state.Handler)
	fmt.Printf("Status:        %s\n", state.Status)
	fmt.Printf("Steps:         %d\n", len(state.Journal))
	fmt.Println("")

	if state.Status == "failed" {
		fmt.Println("Execution FAILED")
		for i, entry := range state.Journal {
			if entry.Error != "" {
				fmt.Printf("Step %d (%s) failed: %s\n", i, entry.StepType, entry.Error)
			}
		}
		return
	}

	fmt.Println("Execution completed successfully!")
	fmt.Println("")
	fmt.Println("Journal:")
	for _, entry := range state.Journal {
		fmt.Printf("  Step %d: %s\n", entry.StepNumber, entry.StepType)

		// Try to decode the response if it's a HelloService response
		if entry.StepType == "HelloService.SayHello" && len(entry.Response) > 0 {
			var helloResp hellopb.HelloResponse
			if err := proto.Unmarshal(entry.Response, &helloResp); err == nil {
				fmt.Printf("    → Response: %s\n", helloResp.Message)
			}
		}
	}
	fmt.Println("========================================")
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
