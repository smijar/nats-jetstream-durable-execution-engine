package execution

import (
	"context"
	"fmt"
	"log"
	"time"

	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/sanjaymijar/my-durable-execution/pb/durable"
	durableCtx "github.com/sanjaymijar/my-durable-execution/pkg/durable"
	"github.com/sanjaymijar/my-durable-execution/pkg/jetstream"
	"google.golang.org/protobuf/proto"
)

// HandlerFunc is the signature for durable execution handlers
type HandlerFunc func(ctx *durableCtx.Context) error

// Processor handles command processing with durable execution
type Processor struct {
	jsClient *jetstream.Client
	handlers map[string]HandlerFunc
	kv       natsjs.KeyValue
}

// NewProcessor creates a new execution processor
func NewProcessor(jsClient *jetstream.Client) (*Processor, error) {
	ctx := context.Background()

	kv, err := jsClient.GetStateKV(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get state KV: %w", err)
	}

	return &Processor{
		jsClient: jsClient,
		handlers: make(map[string]HandlerFunc),
		kv:       kv,
	}, nil
}

// RegisterHandler registers a handler function for a specific handler name
func (p *Processor) RegisterHandler(name string, handler HandlerFunc) {
	p.handlers[name] = handler
}

// Start starts processing commands from the stream
// maxAckPending controls how many workflows can execute concurrently
// ackWait is the timeout for workflow completion before redelivery
func (p *Processor) Start(ctx context.Context, consumerName string, maxAckPending int, ackWait time.Duration) error {
	consumer, err := p.jsClient.SubscribeToCommands(ctx, consumerName, maxAckPending, ackWait)
	if err != nil {
		return fmt.Errorf("failed to subscribe to commands: %w", err)
	}

	log.Printf("Processor started, consuming from %s (max-concurrent=%d, ack-wait=%v)", consumerName, maxAckPending, ackWait)

	// Start consuming messages
	_, err = consumer.Consume(func(msg natsjs.Msg) {
		if err := p.processMessage(ctx, msg); err != nil {
			log.Printf("Error processing message: %v", err)
			// Negative ack with delay for retry
			msg.NakWithDelay(5 * time.Second)
		} else {
			msg.Ack()
		}
	})

	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	// Block until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// processMessage processes a single command message
func (p *Processor) processMessage(ctx context.Context, msg natsjs.Msg) error {
	// Check if message is scheduled for future delivery
	if scheduledTimeStr := msg.Headers().Get("Nats-Scheduled-Time"); scheduledTimeStr != "" {
		scheduledTime, err := time.Parse(time.RFC3339Nano, scheduledTimeStr)
		if err == nil && time.Now().Before(scheduledTime) {
			// Message is not ready yet, requeue with delay
			delay := time.Until(scheduledTime)
			if delay > 30*time.Second {
				delay = 30 * time.Second // Cap retry delay at 30 seconds
			}
			log.Printf("Message scheduled for %v, requeuing (delay: %v)", scheduledTime, delay)
			msg.NakWithDelay(delay)
			return nil
		}
	}

	// Unmarshal command
	var cmd durable.Command
	if err := proto.Unmarshal(msg.Data(), &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	log.Printf("Processing command: invocation_id=%s handler=%s service=%s",
		cmd.InvocationId, cmd.Handler, cmd.Service)

	// Load execution state
	state, err := p.loadState(ctx, cmd.InvocationId)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Check execution status
	if state != nil {
		switch state.Status {
		case "completed":
			log.Printf("Execution already completed: %s", cmd.InvocationId)
			return nil
		case "cancelled":
			log.Printf("Execution cancelled: %s", cmd.InvocationId)
			return nil
		case "paused":
			log.Printf("Execution paused: %s (will retry later)", cmd.InvocationId)
			// Return error so message is redelivered and retried when resumed
			return fmt.Errorf("execution paused: %s", cmd.InvocationId)
		}
	}

	// Initialize state if new execution
	if state == nil {
		state = &durable.ExecutionState{
			InvocationId:    cmd.InvocationId,
			Handler:         cmd.Handler,
			Journal:         []*durable.JournalEntry{},
			Status:          "running",
			CurrentStep:     -1, // -1 means not started yet
			CurrentStepType: "",
			WorkflowState:   make(map[string][]byte),
		}
	}

	// Ensure WorkflowState is initialized (for existing executions from before this feature)
	if state.WorkflowState == nil {
		state.WorkflowState = make(map[string][]byte)
	}

	// Ensure Awakeables is initialized
	if state.Awakeables == nil {
		state.Awakeables = make(map[string]*durable.AwakeableState)
	}

	// Get handler
	handler, exists := p.handlers[cmd.Handler]
	if !exists {
		return fmt.Errorf("handler not found: %s", cmd.Handler)
	}

	// Create durable context for execution
	durableContext := durableCtx.NewContext(ctx, cmd.InvocationId, state.Journal, func(entry *durable.JournalEntry) error {
		// Journal step callback - append to state and save
		state.Journal = append(state.Journal, entry)
		return p.saveState(ctx, state)
	})
	defer durableContext.Close()

	// Load existing workflow state into context
	durableContext.LoadState(state.WorkflowState)

	// Load existing awakeables into context
	durableContext.LoadAwakeables(state.Awakeables)

	// Set callback for tracking current step
	durableContext.SetOnStepStart(func(stepNumber int32, stepType string) error {
		// Update current step in state
		state.CurrentStep = stepNumber
		state.CurrentStepType = stepType
		// Save state to persist current step
		return p.saveState(ctx, state)
	})

	// Set callback for workflow state changes
	durableContext.SetOnStateChange(func(workflowState map[string][]byte) error {
		// Update workflow state and persist
		state.WorkflowState = workflowState
		return p.saveState(ctx, state)
	})

	// Set callback for delayed calls (like Restate's ctx.sendDelayed)
	durableContext.SetOnDelayedCall(func(call *durableCtx.DelayedCall) error {
		// Create a new command for the delayed invocation
		delayedCmd := &durable.Command{
			InvocationId: fmt.Sprintf("%s-delayed-%d", cmd.InvocationId, time.Now().UnixNano()),
			Handler:      call.Handler,
			Args:         call.Args,
			PartitionKey: call.PartitionKey,
		}

		// Publish the delayed command
		return p.SubmitDelayedCommand(ctx, delayedCmd, call.Delay)
	})

	// Set callback for awakeable creation
	durableContext.SetOnAwakeableCreate(func(awakeableID string) error {
		// Update awakeables and persist
		state.Awakeables = durableContext.GetAwakeables()
		return p.saveState(ctx, state)
	})

	// Set input args in context
	durableContext.SetArgs(cmd.Args)

	// Execute handler
	if err := handler(durableContext); err != nil {
		// Check if this is an awakeable suspend error
		if suspendErr, ok := err.(*durableCtx.AwakeableSuspendError); ok {
			log.Printf("Workflow suspended waiting for awakeable: %s (invocation: %s)", suspendErr.AwakeableID, cmd.InvocationId)
			// Return error to requeue message (will be retried when awakeable is resolved)
			return fmt.Errorf("awaiting awakeable %s", suspendErr.AwakeableID)
		}

		log.Printf("Handler execution failed: %v", err)
		state.Status = "failed"
		p.saveState(ctx, state)
		return fmt.Errorf("handler execution failed: %w", err)
	}

	// Extract and save workflow output
	state.CurrentState = durableContext.GetOutput()

	// Mark as completed and clear current step
	state.Status = "completed"
	state.CurrentStep = -1
	state.CurrentStepType = ""
	if err := p.saveState(ctx, state); err != nil {
		return fmt.Errorf("failed to save final state: %w", err)
	}

	log.Printf("Execution completed successfully: %s", cmd.InvocationId)
	return nil
}

// loadState loads execution state from KV bucket
func (p *Processor) loadState(ctx context.Context, invocationID string) (*durable.ExecutionState, error) {
	entry, err := p.kv.Get(ctx, invocationID)
	if err != nil {
		if err == natsjs.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	var state durable.ExecutionState
	if err := proto.Unmarshal(entry.Value(), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// saveState saves execution state to KV bucket atomically
func (p *Processor) saveState(ctx context.Context, state *durable.ExecutionState) error {
	data, err := proto.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	_, err = p.kv.Put(ctx, state.InvocationId, data)
	if err != nil {
		return fmt.Errorf("failed to put state: %w", err)
	}

	return nil
}

// SubmitCommand submits a new command for durable execution
func (p *Processor) SubmitCommand(ctx context.Context, cmd *durable.Command) error {
	data, err := proto.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	if err := p.jsClient.PublishCommand(ctx, cmd.PartitionKey, data); err != nil {
		return fmt.Errorf("failed to publish command: %w", err)
	}

	log.Printf("Command submitted: invocation_id=%s handler=%s", cmd.InvocationId, cmd.Handler)
	return nil
}

// SubmitDelayedCommand submits a command with delayed execution
// Similar to Restate's ctx.sendDelayed()
func (p *Processor) SubmitDelayedCommand(ctx context.Context, cmd *durable.Command, delay time.Duration) error {
	data, err := proto.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal delayed command: %w", err)
	}

	if err := p.jsClient.PublishDelayedCommand(ctx, cmd.PartitionKey, data, delay); err != nil {
		return fmt.Errorf("failed to publish delayed command: %w", err)
	}

	log.Printf("Delayed command submitted: invocation_id=%s handler=%s delay=%v", cmd.InvocationId, cmd.Handler, delay)
	return nil
}

// ResolveAwakeable resolves an awakeable with a result or error
// This triggers the workflow to resume execution
func (p *Processor) ResolveAwakeable(ctx context.Context, awakeableID string, result []byte, errorMsg string) error {
	// Find the invocation that owns this awakeable
	// We need to scan KV for all executions to find the one with this awakeable
	keys, err := p.kv.Keys(ctx)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	var targetInvocationID string
	for _, key := range keys {
		entry, err := p.kv.Get(ctx, key)
		if err != nil {
			continue
		}

		var state durable.ExecutionState
		if err := proto.Unmarshal(entry.Value(), &state); err != nil {
			continue
		}

		if state.Awakeables != nil {
			if awakeable, exists := state.Awakeables[awakeableID]; exists {
				targetInvocationID = awakeable.InvocationId
				break
			}
		}
	}

	if targetInvocationID == "" {
		return fmt.Errorf("awakeable not found: %s", awakeableID)
	}

	// Load the execution state
	state, err := p.loadState(ctx, targetInvocationID)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Resolve the awakeable
	awakeable := state.Awakeables[awakeableID]
	awakeable.Resolved = true
	awakeable.Result = result
	awakeable.Error = errorMsg

	// Save the updated state
	if err := p.saveState(ctx, state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	log.Printf("Awakeable resolved: id=%s invocation=%s", awakeableID, targetInvocationID)
	return nil
}
