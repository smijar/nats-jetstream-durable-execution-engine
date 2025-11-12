package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	ticketpb "github.com/sanjaymijar/my-durable-execution/pb/ticket"
	"github.com/sanjaymijar/my-durable-execution/pb/durable"
	"github.com/sanjaymijar/my-durable-execution/pkg/api"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
	durableSDK "github.com/sanjaymijar/my-durable-execution/pkg/durable"
	"github.com/sanjaymijar/my-durable-execution/pkg/execution"
	"github.com/sanjaymijar/my-durable-execution/pkg/jetstream"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Parse command-line flags
	maxConcurrent := flag.Int("max-concurrent", 100, "Maximum number of concurrent workflow executions (MaxAckPending)")
	ackWaitSeconds := flag.Int("ack-wait", 300, "Timeout in seconds for workflow completion before redelivery (default: 300s / 5min)")
	httpPort := flag.Int("http-port", 8080, "HTTP API port for workflow queries (0 to disable)")
	flag.Parse()

	ackWait := time.Duration(*ackWaitSeconds) * time.Second

	log.Println("Starting Durable Execution Processor...")
	log.Printf("Configuration: max-concurrent=%d, ack-wait=%v, http-port=%d", *maxConcurrent, ackWait, *httpPort)

	// Connect to NATS
	natsURL := getEnv("NATS_URL", "nats://127.0.0.1:4322")
	jsClient, err := jetstream.NewClient(natsURL)
	if err != nil {
		log.Fatalf("Failed to create JetStream client: %v", err)
	}
	defer jsClient.Close()

	ctx := context.Background()

	// Setup streams and KV buckets
	log.Println("Setting up JetStream streams...")
	if err := jsClient.SetupStreams(ctx); err != nil {
		log.Fatalf("Failed to setup streams: %v", err)
	}

	log.Println("Setting up KV buckets...")
	if err := jsClient.SetupKVBuckets(ctx); err != nil {
		log.Fatalf("Failed to setup KV buckets: %v", err)
	}

	// Create a registry for our services and their invokers
	serviceRegistry := durableSDK.NewServiceRegistry()
	serviceRegistry.Register("HelloService", durableSDK.NewGRPCInvoker("127.0.0.1:9090"))
	serviceRegistry.Register("TicketService", durableSDK.NewGRPCInvoker("127.0.0.1:9091"))
	defer serviceRegistry.Close()

	// Create processor
	processor, err := execution.NewProcessor(jsClient, serviceRegistry)
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	// Register workflows using Restate-style API
	registry := durableSDK.NewWorkflowRegistry()

	// Register HelloWorkflow (for backward compatibility with old examples)
	registry.Register(HelloWorkflow)

	// Register GreetingWorkflow (for restate-style example)
	registry.Register(GreetingWorkflow)

	// Register GreetManyWorkflow (for composition examples)
	registry.Register(GreetManyWorkflow)

	// Register MultiServiceWorkflow (for multi-service invocation demo)
	registry.Register(MultiServiceWorkflow)

	// Register AsyncWorkflow (for concurrent workflow invocations)
	registry.Register(AsyncWorkflow)

	// Register FlakyWorkflow (for retry/failure handling demo)
	registry.Register(FlakyWorkflow)

	// Register SlowWorkflow (for pause/cancel demo)
	registry.Register(SlowWorkflow)

	// Register AddTicketToCartWorkflow (Restate-style with state management)
	registry.Register(AddTicketToCartWorkflow)

	// Register ExpireTicketWorkflow (for delayed execution demo)
	registry.Register(ExpireTicketWorkflow)

	// Register SchedulerWorkflow (for delayed execution example)
	registry.Register(SchedulerWorkflow)

	// Register DelayedGreetingWorkflow (for delayed execution example)
	registry.Register(DelayedGreetingWorkflow)

	// Register ExpenseApprovalWorkflow (for awakeable example)
	registry.Register(ExpenseApprovalWorkflow)

	// Register all workflows from registry into processor
	for name, handler := range registry.GetAll() {
		log.Printf("Registering workflow: %s", name)
		processor.RegisterHandler(name, handler)
	}

	// Submit a test command (optional - for demonstration)
	if getEnv("SUBMIT_TEST_COMMAND", "false") == "true" {
		if err := submitTestCommand(ctx, processor); err != nil {
			log.Printf("Warning: Failed to submit test command: %v", err)
		}
	}

	// Start HTTP API server (if enabled)
	processorCtx, cancelProcessor := context.WithCancel(ctx)
	defer cancelProcessor()

	if *httpPort > 0 {
		// Create a client for the API server to use
		apiClient, err := client.NewClient(natsURL)
		if err != nil {
			log.Fatalf("Failed to create API client: %v", err)
		}
		defer apiClient.Close()

		// Create and start HTTP API server
		apiServer := api.NewServer(*httpPort, apiClient, processor)
		go func() {
			if err := apiServer.Start(processorCtx); err != nil {
				log.Printf("HTTP API server error: %v", err)
			}
		}()
	}

	// Start processor in a goroutine
	go func() {
		if err := processor.Start(processorCtx, "processor-1", *maxConcurrent, ackWait); err != nil {
			if err != context.Canceled {
				log.Fatalf("Processor error: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down processor...")
	cancelProcessor()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	log.Println("Processor stopped")
}

// ====================================================================================
// Workflow Definitions (Restate-style)
// ====================================================================================

// HelloWorkflowInput represents input for HelloWorkflow
type HelloWorkflowInput struct {
	Name string `json:"name"`
}

// HelloWorkflow - for backward compatibility with older examples
var HelloWorkflow = durableSDK.NewWorkflow("hello_workflow",
	func(ctx *durableSDK.Context, input HelloWorkflowInput) (string, error) {
		log.Printf("Executing HelloWorkflow: invocation_id=%s name=%s", ctx.InvocationID(), input.Name)

		req := &hellopb.HelloRequest{Name: input.Name}
		resp := &hellopb.HelloResponse{}

		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		log.Printf("HelloWorkflow completed: message=%s", resp.Message)
		return resp.Message, nil
	})

// GreetingWorkflow - for restate-style example
var GreetingWorkflow = durableSDK.NewWorkflow("greeting",
	func(ctx *durableSDK.Context, name string) (string, error) {
		log.Printf("Executing GreetingWorkflow: invocation_id=%s", ctx.InvocationID())

		req := &hellopb.HelloRequest{Name: name}
		resp := &hellopb.HelloResponse{}

		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		log.Printf("GreetingWorkflow completed: message=%s", resp.Message)
		return resp.Message, nil
	})

// GreetManyWorkflow - composition example
var GreetManyWorkflow = durableSDK.NewWorkflow("greet_many",
	func(ctx *durableSDK.Context, names []string) ([]string, error) {
		log.Printf("Executing GreetManyWorkflow: invocation_id=%s with %d names", ctx.InvocationID(), len(names))

		results := make([]string, 0, len(names))

		for _, name := range names {
			req := &hellopb.HelloRequest{Name: name}
			resp := &hellopb.HelloResponse{}

			if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
				return nil, err
			}

			results = append(results, resp.Message)
		}

		log.Printf("GreetManyWorkflow completed: processed %d greetings", len(results))
		return results, nil
	})

// MultiServiceWorkflow - demonstrates multiple sequential service calls
var MultiServiceWorkflow = durableSDK.NewWorkflow("multi_service_demo",
	func(ctx *durableSDK.Context, topic string) (string, error) {
		log.Printf("Executing MultiServiceWorkflow: invocation_id=%s topic=%s", ctx.InvocationID(), topic)

		// Call 1: English greeting
		req1 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (English)", topic)}
		resp1 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req1, resp1); err != nil {
			return "", err
		}

		// Call 2: Spanish greeting
		req2 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (Spanish)", topic)}
		resp2 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req2, resp2); err != nil {
			return "", err
		}

		// Call 3: French greeting
		req3 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s (French)", topic)}
		resp3 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req3, resp3); err != nil {
			return "", err
		}

		// Call 4: Summary
		req4 := &hellopb.HelloRequest{Name: "Summary"}
		resp4 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req4, resp4); err != nil {
			return "", err
		}

		summary := fmt.Sprintf(
			"Multi-Service Workflow Results:\n"+
				"1️⃣  %s\n"+
				"2️⃣  %s\n"+
				"3️⃣  %s\n"+
				"📊 %s\n"+
				"Total service calls: 4",
			resp1.Message, resp2.Message, resp3.Message, resp4.Message,
		)

		log.Printf("MultiServiceWorkflow completed with 4 service calls")
		return summary, nil
	})

// AsyncWorkflow - demonstrates a 3-step workflow for concurrent invocations
var AsyncWorkflow = durableSDK.NewWorkflow("async_workflow",
	func(ctx *durableSDK.Context, userID string) (string, error) {
		log.Printf("Executing AsyncWorkflow: invocation_id=%s userID=%s", ctx.InvocationID(), userID)

		// Step 1: Synchronous greeting
		req1 := &hellopb.HelloRequest{Name: fmt.Sprintf("User:%s", userID)}
		resp1 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req1, resp1); err != nil {
			return "", err
		}

		// Step 2: Async-style call
		req2 := &hellopb.HelloRequest{Name: fmt.Sprintf("Async:%s", userID)}
		resp2 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req2, resp2); err != nil {
			return "", err
		}

		// Step 3: Final confirmation
		req3 := &hellopb.HelloRequest{Name: fmt.Sprintf("Confirm:%s", userID)}
		resp3 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req3, resp3); err != nil {
			return "", err
		}

		result := fmt.Sprintf(
			"AsyncWorkflow[%s] completed:\n  Step1: %s\n  Step2: %s\n  Step3: %s",
			userID, resp1.Message, resp2.Message, resp3.Message,
		)

		log.Printf("AsyncWorkflow completed for user %s", userID)
		return result, nil
	})

// FlakyWorkflow - simulates a workflow that might fail (for retry demo)
var FlakyWorkflow = durableSDK.NewWorkflow("flaky_workflow",
	func(ctx *durableSDK.Context, input string) (string, error) {
		log.Printf("Executing FlakyWorkflow: invocation_id=%s input=%s", ctx.InvocationID(), input)

		// Simulate transient failure on first attempt
		if strings.Contains(input, "flaky") && !strings.Contains(input, "retry") {
			log.Printf("FlakyWorkflow simulating failure for: %s", input)
			return "", fmt.Errorf("transient error: service temporarily unavailable for %s", input)
		}

		// Call service
		req := &hellopb.HelloRequest{Name: input}
		resp := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		log.Printf("FlakyWorkflow succeeded for: %s", input)
		return resp.Message, nil
	})

// SlowWorkflow - multi-step workflow that can be paused/cancelled
var SlowWorkflow = durableSDK.NewWorkflow("slow_workflow",
	func(ctx *durableSDK.Context, taskName string) (string, error) {
		log.Printf("Executing SlowWorkflow: invocation_id=%s task=%s", ctx.InvocationID(), taskName)
		results := []string{}

		// Step 1
		log.Printf("SlowWorkflow[%s] Step 1/3...", taskName)
		req1 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s-step1", taskName)}
		resp1 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req1, resp1); err != nil {
			return "", err
		}
		results = append(results, resp1.Message)

		// Step 2
		log.Printf("SlowWorkflow[%s] Step 2/3...", taskName)
		req2 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s-step2", taskName)}
		resp2 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req2, resp2); err != nil {
			return "", err
		}
		results = append(results, resp2.Message)

		// Step 3
		log.Printf("SlowWorkflow[%s] Step 3/3...", taskName)
		req3 := &hellopb.HelloRequest{Name: fmt.Sprintf("%s-step3", taskName)}
		resp3 := &hellopb.HelloResponse{}
		if err := ctx.DurableCall("HelloService", "SayHello", req3, resp3); err != nil {
			return "", err
		}
		results = append(results, resp3.Message)

		result := fmt.Sprintf("SlowWorkflow[%s] completed all 3 steps", taskName)
		log.Printf("SlowWorkflow completed for: %s", taskName)
		return result, nil
	})

// AddTicketToCartInput represents input for add ticket to cart workflow
type AddTicketToCartInput struct {
	UserID   string `json:"user_id"`
	TicketID string `json:"ticket_id"`
}

// ExpireTicketInput represents input for expire ticket workflow
type ExpireTicketInput struct {
	UserID   string `json:"user_id"`
	TicketID string `json:"ticket_id"`
}

// AddTicketToCartWorkflow - Restate-style workflow with state management
var AddTicketToCartWorkflow = durableSDK.NewWorkflow("add_ticket_to_cart",
	func(ctx *durableSDK.Context, input AddTicketToCartInput) (bool, error) {
		log.Printf("🛒 Adding ticket to cart: user=%s, ticket=%s", input.UserID, input.TicketID)

		// Step 1: Try to reserve the ticket
		req := &ticketpb.ReserveRequest{TicketId: input.TicketID}
		resp := &ticketpb.ReserveResponse{}

		if err := ctx.DurableCall("TicketService", "Reserve", req, resp); err != nil {
			log.Printf("❌ Failed to call TicketService: %v", err)
			return false, err
		}

		log.Printf("📞 Reserve response: success=%v, message=%s", resp.Success, resp.Message)

		if resp.Success {
			// Step 2: Update workflow state (cart contents)
			var cart []string
			err := ctx.Get("cart", &cart)
			if err != nil {
				// Cart doesn't exist yet, start with empty
				cart = []string{}
				log.Printf("📦 Cart doesn't exist yet, creating new cart")
			} else {
				log.Printf("📦 Current cart: %v", cart)
			}

			// Add ticket to cart
			cart = append(cart, input.TicketID)

			// Save updated cart
			if err := ctx.Set("cart", cart); err != nil {
				log.Printf("❌ Failed to update cart: %v", err)
				return false, err
			}

			log.Printf("✅ Ticket added to cart! New cart: %v", cart)

			// Step 3: Schedule expiry timer (15 minutes)
			// Similar to Restate's ctx.sendDelayed()
			expireInput := ExpireTicketInput{
				UserID:   input.UserID,
				TicketID: input.TicketID,
			}
			if err := ctx.SendDelayed("expire_ticket", expireInput, 15*time.Minute, input.UserID); err != nil {
				log.Printf("⚠️  Failed to schedule expiry timer: %v", err)
				// Don't fail the workflow, just log the error
			} else {
				log.Printf("⏱️  Scheduled expiry timer for ticket %s in 15 minutes", input.TicketID)
			}
		} else {
			log.Printf("❌ Ticket reservation failed: %s", resp.Message)
		}

		return resp.Success, nil
	})

// ExpireTicketWorkflow - Releases a ticket after expiry time
var ExpireTicketWorkflow = durableSDK.NewWorkflow("expire_ticket",
	func(ctx *durableSDK.Context, input ExpireTicketInput) (bool, error) {
		log.Printf("⏱️  Expiring ticket: user=%s, ticket=%s", input.UserID, input.TicketID)

		// Step 1: Remove ticket from cart
		var cart []string
		err := ctx.Get("cart", &cart)
		if err != nil {
			log.Printf("⚠️  Cart not found for user %s", input.UserID)
			// Cart doesn't exist, nothing to expire
			return false, nil
		}

		// Find and remove the ticket
		found := false
		newCart := []string{}
		for _, ticket := range cart {
			if ticket == input.TicketID {
				found = true
				continue // Skip this ticket
			}
			newCart = append(newCart, ticket)
		}

		if found {
			// Update cart
			if err := ctx.Set("cart", newCart); err != nil {
				log.Printf("❌ Failed to update cart: %v", err)
				return false, err
			}
			log.Printf("📦 Removed ticket from cart. New cart: %v", newCart)
		} else {
			log.Printf("⚠️  Ticket %s not found in cart", input.TicketID)
		}

		// Step 2: Release the ticket reservation
		releaseReq := &ticketpb.ReleaseRequest{TicketId: input.TicketID}
		releaseResp := &ticketpb.ReleaseResponse{}

		if err := ctx.DurableCall("TicketService", "Release", releaseReq, releaseResp); err != nil {
			log.Printf("❌ Failed to release ticket: %v", err)
			return false, err
		}

		if releaseResp.Success {
			log.Printf("✅ Ticket %s expired and released successfully", input.TicketID)
		} else {
			log.Printf("⚠️  Failed to release ticket %s", input.TicketID)
		}

		return releaseResp.Success, nil
	})

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
var SchedulerWorkflow = durableSDK.NewWorkflow("scheduler",
	func(ctx *durableSDK.Context, input SchedulerInput) (string, error) {
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
var DelayedGreetingWorkflow = durableSDK.NewWorkflow("delayed_greeting",
	func(ctx *durableSDK.Context, input GreetingInput) (string, error) {
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

// ExpenseRequest represents an expense approval request
type ExpenseRequest struct {
	EmployeeID  string  `json:"employee_id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
}

// ApprovalResult represents the result of an approval
type ApprovalResult struct {
	Approved bool   `json:"approved"`
	Comments string `json:"comments"`
}

// ExpenseApprovalWorkflow - Demonstrates awakeables for external approvals
var ExpenseApprovalWorkflow = durableSDK.NewWorkflow("expense_approval",
	func(ctx *durableSDK.Context, request ExpenseRequest) (string, error) {
		log.Printf("💰 Processing expense request: employee=%s, amount=$%.2f, desc=%s",
			request.EmployeeID, request.Amount, request.Description)

		// Step 1: Log the request
		req := &hellopb.HelloRequest{Name: fmt.Sprintf("Expense Request from %s", request.EmployeeID)}
		resp := &hellopb.HelloResponse{}

		if err := ctx.DurableCall("HelloService", "SayHello", req, resp); err != nil {
			return "", err
		}

		log.Printf("📝 Request logged: %s", resp.Message)

		// Step 2: Create an awakeable for manager approval
		awakeableID := fmt.Sprintf("approval-%s", uuid.New().String())

		log.Printf("⏳ Waiting for manager approval (awakeable ID: %s)", awakeableID)
		log.Printf("   Manager can approve by calling:")
		log.Printf("   curl -X POST http://localhost:8080/api/awakeables/%s/resolve \\", awakeableID)
		log.Printf("     -H 'Content-Type: application/json' \\")
		log.Printf("     -d '{\"result\": \"{\\\"approved\\\": true, \\\"comments\\\": \\\"Looks good!\\\"}\"}'")
		log.Printf("")
		log.Printf("   Or reject by calling:")
		log.Printf("   curl -X POST http://localhost:8080/api/awakeables/%s/reject \\", awakeableID)
		log.Printf("     -H 'Content-Type: application/json' \\")
		log.Printf("     -d '{\"error\": \"Amount too high\"}'")

		// Wait for external approval (this suspends the workflow)
		resultBytes, err := ctx.Awakeable(awakeableID)
		if err != nil {
			log.Printf("❌ Expense rejected: %v", err)
			return fmt.Sprintf("Expense rejected: %v", err), err
		}

		// Parse approval result
		var approval ApprovalResult
		if err := json.Unmarshal(resultBytes, &approval); err != nil {
			return "", fmt.Errorf("failed to parse approval result: %w", err)
		}

		log.Printf("📨 Received approval decision: approved=%v, comments=%s", approval.Approved, approval.Comments)

		// Step 3: Process based on approval
		var finalMessage string
		if approval.Approved {
			// Expense approved - notify employee
			notifyReq := &hellopb.HelloRequest{Name: fmt.Sprintf("Approved: %s", request.EmployeeID)}
			notifyResp := &hellopb.HelloResponse{}

			if err := ctx.DurableCall("HelloService", "SayHello", notifyReq, notifyResp); err != nil {
				return "", err
			}

			finalMessage = fmt.Sprintf("✅ Expense of $%.2f approved for %s. Comments: %s",
				request.Amount, request.EmployeeID, approval.Comments)
		} else {
			finalMessage = fmt.Sprintf("❌ Expense of $%.2f rejected for %s. Comments: %s",
				request.Amount, request.EmployeeID, approval.Comments)
		}

		log.Printf("%s", finalMessage)
		return finalMessage, nil
	})

// submitTestCommand submits a test command for demonstration
func submitTestCommand(ctx context.Context, processor *execution.Processor) error {
	// Marshal the input struct to JSON
	input := HelloWorkflowInput{Name: "TestClient"}
	args, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal test command args: %w", err)
	}

	cmd := &durable.Command{
		InvocationId: uuid.New().String(),
		Handler:      "hello_workflow",
		Service:      "HelloService",
		Args:         args,
		PartitionKey: "default",
		Sequence:     1,
	}

	log.Printf("Submitting test command: %s", cmd.InvocationId)
	return processor.SubmitCommand(ctx, cmd)
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
