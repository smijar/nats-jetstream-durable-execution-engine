package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	ticketpb "github.com/sanjaymijar/my-durable-execution/pb/ticket"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
	"github.com/sanjaymijar/my-durable-execution/pkg/durable"
)

// ====================================================================================
// Restate-style Workflow: Add Ticket to Cart
// ====================================================================================

// AddTicketToCartInput represents the workflow input
type AddTicketToCartInput struct {
	UserID   string `json:"user_id"`
	TicketID string `json:"ticket_id"`
}

// ExpireTicketInput represents input for expire ticket workflow
type ExpireTicketInput struct {
	UserID   string `json:"user_id"`
	TicketID string `json:"ticket_id"`
}

// AddTicketToCart workflow - Restate-style implementation
// Equivalent to the Restate example:
//
//	async function addTicketToCart(
//	  ctx: restate.RpcContext,
//	  userId: string,
//	  ticketId: string
//	) {
//	  // try to reserve the ticket
//	  const success = await ctx.rpc(ticketApi).reserve(ticketId);
//
//	  if (success) {
//	    // update the local state (cart contents)
//	    const tickets = (await ctx.get<string[]>("cart")) ?? [];
//	    tickets.push(ticketId);
//	    ctx.set("cart", tickets);
//
//	    // schedule expiry timer (future feature)
//	    // ctx.sendDelayed(sessionApi, minutes(15)).expireTicket(userId, ticketId);
//	  }
//
//	  return success;
//	}
var AddTicketToCartWorkflow = durable.NewWorkflow("add_ticket_to_cart",
	func(ctx *durable.Context, input AddTicketToCartInput) (bool, error) {
		log.Printf("🛒 Adding ticket to cart: user=%s, ticket=%s", input.UserID, input.TicketID)

		// ==============================================================
		// Step 1: Try to reserve the ticket (durable RPC call)
		// ==============================================================
		req := &ticketpb.ReserveRequest{TicketId: input.TicketID}
		resp := &ticketpb.ReserveResponse{}

		if err := ctx.DurableCall("TicketService", "Reserve", req, resp); err != nil {
			log.Printf("❌ Failed to call TicketService: %v", err)
			return false, err
		}

		log.Printf("📞 Reserve response: success=%v, message=%s", resp.Success, resp.Message)

		if resp.Success {
			// ==============================================================
			// Step 2: Update the workflow state (cart contents)
			// This is like Restate's ctx.get() / ctx.set()
			// ==============================================================

			// Get current cart (or empty array if doesn't exist)
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

			// ==============================================================
			// Step 3: Schedule expiry timer (15 minutes)
			// Similar to Restate's ctx.sendDelayed()
			// ==============================================================
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

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("========================================")
	fmt.Println("Ticket Cart Example (Restate-style)")
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
	// Scenario 1: Add first ticket to cart
	// ==============================================================
	fmt.Println("📤 Scenario 1: Adding first ticket to cart...")
	fmt.Println()

	input1 := AddTicketToCartInput{
		UserID:   "user-123",
		TicketID: "TICKET-001",
	}

	inputBytes1, _ := json.Marshal(input1)
	success1, err := client.InvokeWorkflow[AddTicketToCartInput, bool](c, ctx, AddTicketToCartWorkflow, input1)
	if err != nil {
		log.Printf("Failed to invoke workflow: %v", err)
	} else {
		fmt.Printf("✅ Result: success=%v\n", success1)
	}

	fmt.Println()

	// ==============================================================
	// Scenario 2: Add second ticket to cart (state persists!)
	// ==============================================================
	fmt.Println("📤 Scenario 2: Adding second ticket to same cart...")
	fmt.Println()

	input2 := AddTicketToCartInput{
		UserID:   "user-123",
		TicketID: "TICKET-002",
	}

	inputBytes2, _ := json.Marshal(input2)
	success2, err := client.InvokeWorkflow[AddTicketToCartInput, bool](c, ctx, AddTicketToCartWorkflow, input2)
	if err != nil {
		log.Printf("Failed to invoke workflow: %v", err)
	} else {
		fmt.Printf("✅ Result: success=%v\n", success2)
	}

	fmt.Println()

	// ==============================================================
	// Scenario 3: Try to add already reserved ticket (should fail)
	// ==============================================================
	fmt.Println("📤 Scenario 3: Trying to add already reserved ticket...")
	fmt.Println()

	input3 := AddTicketToCartInput{
		UserID:   "user-456",
		TicketID: "TICKET-001", // Already reserved by user-123
	}

	inputBytes3, _ := json.Marshal(input3)
	success3, err := client.InvokeWorkflow[AddTicketToCartInput, bool](c, ctx, AddTicketToCartWorkflow, input3)
	if err != nil {
		log.Printf("Failed to invoke workflow: %v", err)
	} else {
		fmt.Printf("✅ Result: success=%v (expected: false)\n", success3)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("✓ Test completed!")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Key Features Demonstrated:")
	fmt.Println("  ✅ Durable RPC calls (ctx.DurableCall)")
	fmt.Println("  ✅ Workflow-scoped state (ctx.Get/Set)")
	fmt.Println("  ✅ State persistence across invocations")
	fmt.Println("  ✅ Exactly-once semantics")
	fmt.Println("  ✅ Delayed execution (ctx.SendDelayed)")
	fmt.Println()
	fmt.Println("Note: Tickets will expire and be released after 15 minutes if not checked out.")

	// Suppress unused variable warnings
	_ = inputBytes1
	_ = inputBytes2
	_ = inputBytes3
}
