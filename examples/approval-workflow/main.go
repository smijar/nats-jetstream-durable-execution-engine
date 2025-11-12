package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
	"github.com/sanjaymijar/my-durable-execution/pkg/durable"
)

// ====================================================================================
// Awakeable Example: Expense Approval Workflow
// ====================================================================================

// ExpenseRequest represents an expense approval request
type ExpenseRequest struct {
	EmployeeID string  `json:"employee_id"`
	Amount     float64 `json:"amount"`
	Description string `json:"description"`
}

// ApprovalResult represents the result of an approval
type ApprovalResult struct {
	Approved bool   `json:"approved"`
	Comments string `json:"comments"`
}

// ExpenseApprovalWorkflow - Demonstrates awakeables for external approvals
var ExpenseApprovalWorkflow = durable.NewWorkflow("expense_approval",
	func(ctx *durable.Context, request ExpenseRequest) (string, error) {
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
		// Generate a unique awakeable ID
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
			// Approval was rejected or failed
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
			// Expense rejected
			finalMessage = fmt.Sprintf("❌ Expense of $%.2f rejected for %s. Comments: %s",
				request.Amount, request.EmployeeID, approval.Comments)
		}

		log.Printf("%s", finalMessage)
		return finalMessage, nil
	})

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("========================================")
	fmt.Println("Expense Approval Workflow (Awakeables)")
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
	// Scenario: Submit an expense for approval
	// ==============================================================
	fmt.Println("📤 Submitting expense for approval...")
	fmt.Println()

	expense := ExpenseRequest{
		EmployeeID:  "john.doe",
		Amount:      1250.00,
		Description: "Conference travel expenses",
	}

	fmt.Printf("Employee: %s\n", expense.EmployeeID)
	fmt.Printf("Amount: $%.2f\n", expense.Amount)
	fmt.Printf("Description: %s\n", expense.Description)
	fmt.Println()

	// Invoke workflow (this will block until approved)
	fmt.Println("⏳ Workflow submitted and waiting for approval...")
	fmt.Println("   (The workflow is now suspended, waiting for external approval)")
	fmt.Println()
	fmt.Println("To approve or reject this expense, check the processor logs for the curl command.")
	fmt.Println("The workflow will resume once you make the HTTP call.")
	fmt.Println()

	result, err := client.InvokeWorkflow[ExpenseRequest, string](c, ctx, ExpenseApprovalWorkflow, expense)
	if err != nil {
		log.Printf("Workflow failed: %v", err)
	} else {
		fmt.Printf("✅ Workflow completed: %s\n", result)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("✓ Example completed!")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Key Features Demonstrated:")
	fmt.Println("  ✅ Awakeables for external callbacks (ctx.Awakeable)")
	fmt.Println("  ✅ Workflow suspension until resolved")
	fmt.Println("  ✅ HTTP API for resolving awakeables")
	fmt.Println("  ✅ Durable RPC calls (ctx.DurableCall)")
	fmt.Println("  ✅ Exactly-once semantics")
	fmt.Println()
	fmt.Println("Use Cases:")
	fmt.Println("  ✅ Manager approvals")
	fmt.Println("  ✅ Payment webhooks (Stripe, PayPal)")
	fmt.Println("  ✅ External API callbacks")
	fmt.Println("  ✅ Human-in-the-loop workflows")
}
