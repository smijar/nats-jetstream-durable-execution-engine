package durable

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sanjaymijar/my-durable-execution/pb/durable"
	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	ticketpb "github.com/sanjaymijar/my-durable-execution/pb/ticket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// DelayedCall represents a delayed workflow invocation
type DelayedCall struct {
	Handler      string
	Args         []byte
	Delay        time.Duration
	PartitionKey string
}

// AwakeableSuspendError is returned when an awakeable is not yet resolved
type AwakeableSuspendError struct {
	AwakeableID string
}

func (e *AwakeableSuspendError) Error() string {
	return fmt.Sprintf("awakeable %s not resolved yet, suspending workflow", e.AwakeableID)
}

// Context provides the durable execution SDK for handlers
type Context struct {
	ctx               context.Context
	invocationID      string
	journal           []*durable.JournalEntry
	currentStep       int32
	isReplaying       bool
	grpcConns         map[string]*grpc.ClientConn
	onJournalStep     func(*durable.JournalEntry) error
	onStepStart       func(stepNumber int32, stepType string) error           // Called when a new step starts executing
	onStateChange     func(state map[string][]byte) error                     // Called when workflow state changes
	onDelayedCall     func(call *DelayedCall) error                           // Called when scheduling delayed invocation
	onAwakeableCreate func(awakeableID string) error                          // Called when creating an awakeable
	state             map[string][]byte                                       // Workflow-scoped state
	awakeables        map[string]*durable.AwakeableState                      // Awakeables for this workflow
	args              []byte                                                   // Workflow input arguments
	output            []byte                                                   // Workflow output result
}

// NewContext creates a new durable execution context
func NewContext(ctx context.Context, invocationID string, journal []*durable.JournalEntry, onJournalStep func(*durable.JournalEntry) error) *Context {
	return &Context{
		ctx:           ctx,
		invocationID:  invocationID,
		journal:       journal,
		currentStep:   0,
		isReplaying:   len(journal) > 0,
		grpcConns:     make(map[string]*grpc.ClientConn),
		onJournalStep: onJournalStep,
		state:         make(map[string][]byte),
		awakeables:    make(map[string]*durable.AwakeableState),
	}
}

// DurableCall invokes a service durably with deterministic replay
// During replay, it returns cached results from the journal
// During new execution, it calls the service and journals the result
func (dc *Context) DurableCall(serviceName string, method string, req proto.Message, resp proto.Message) error {
	stepType := fmt.Sprintf("%s.%s", serviceName, method)

	// Notify about step start (for tracking current step in ExecutionState)
	if dc.onStepStart != nil {
		if err := dc.onStepStart(dc.currentStep, stepType); err != nil {
			// Log but don't fail the workflow
			fmt.Printf("Warning: onStepStart callback failed: %v\n", err)
		}
	}

	// Check if we're replaying and this step exists in the journal
	if dc.isReplaying && int(dc.currentStep) < len(dc.journal) {
		entry := dc.journal[dc.currentStep]
		dc.currentStep++

		// Verify step type matches
		if entry.StepType != stepType {
			return fmt.Errorf("step type mismatch: expected %s, got %s", stepType, entry.StepType)
		}

		// Check if the step errored
		if entry.Error != "" {
			return fmt.Errorf("replayed error: %s", entry.Error)
		}

		// Unmarshal cached response
		if err := proto.Unmarshal(entry.Response, resp); err != nil {
			return fmt.Errorf("failed to unmarshal cached response: %w", err)
		}

		return nil
	}

	// We're past replay or this is a new execution - execute the call
	dc.isReplaying = false

	// Serialize request
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Execute the actual call (this would be a real gRPC call in production)
	// For now, we'll create a journal entry structure
	entry := &durable.JournalEntry{
		StepNumber: dc.currentStep,
		StepType:   stepType,
		Request:    reqBytes,
	}

	// Make the actual gRPC call
	conn, err := dc.getOrCreateConnection(serviceName)
	if err != nil {
		entry.Error = err.Error()
		if dc.onJournalStep != nil {
			dc.onJournalStep(entry)
		}
		dc.currentStep++
		return err
	}

	// Invoke the method using reflection or a service-specific client
	// For this implementation, we'll use a generic approach
	respBytes, err := dc.invokeGRPCMethod(conn, serviceName, method, reqBytes)
	if err != nil {
		entry.Error = err.Error()
		if dc.onJournalStep != nil {
			dc.onJournalStep(entry)
		}
		dc.currentStep++
		return err
	}

	// Unmarshal response
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		entry.Error = fmt.Sprintf("failed to unmarshal response: %v", err)
		if dc.onJournalStep != nil {
			dc.onJournalStep(entry)
		}
		dc.currentStep++
		return err
	}

	// Journal the successful result
	entry.Response = respBytes
	if dc.onJournalStep != nil {
		if err := dc.onJournalStep(entry); err != nil {
			return fmt.Errorf("failed to journal step: %w", err)
		}
	}

	dc.currentStep++
	return nil
}

// getOrCreateConnection gets or creates a gRPC connection to a service
func (dc *Context) getOrCreateConnection(serviceName string) (*grpc.ClientConn, error) {
	if conn, exists := dc.grpcConns[serviceName]; exists {
		return conn, nil
	}

	// Default to localhost for services
	// In production, this would use service discovery
	serviceAddr := fmt.Sprintf("127.0.0.1:%d", getServicePort(serviceName))

	conn, err := grpc.Dial(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", serviceName, err)
	}

	dc.grpcConns[serviceName] = conn
	return conn, nil
}

// invokeGRPCMethod invokes a gRPC method generically
func (dc *Context) invokeGRPCMethod(conn *grpc.ClientConn, service, method string, reqBytes []byte) ([]byte, error) {
	// This is a simplified implementation
	// In a real system, you'd use the generated gRPC client stubs
	// or a reflection-based invoker

	// For the HelloService, we'll handle it specifically
	if service == "HelloService" && method == "SayHello" {
		return dc.invokeHelloService(conn, reqBytes)
	}

	// For the TicketService
	if service == "TicketService" {
		switch method {
		case "Reserve":
			return dc.invokeTicketReserve(conn, reqBytes)
		case "Release":
			return dc.invokeTicketRelease(conn, reqBytes)
		}
	}

	return nil, fmt.Errorf("unsupported service: %s.%s", service, method)
}

// invokeHelloService invokes the HelloService.SayHello method
func (dc *Context) invokeHelloService(conn *grpc.ClientConn, reqBytes []byte) ([]byte, error) {
	// Create the HelloService client
	client := hellopb.NewHelloServiceClient(conn)

	// Unmarshal the request
	req := &hellopb.HelloRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal HelloRequest: %w", err)
	}

	// Make the actual gRPC call
	resp, err := client.SayHello(dc.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("SayHello RPC failed: %w", err)
	}

	// Marshal the response
	respBytes, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal HelloResponse: %w", err)
	}

	return respBytes, nil
}

// invokeTicketReserve invokes TicketService.Reserve
func (dc *Context) invokeTicketReserve(conn *grpc.ClientConn, reqBytes []byte) ([]byte, error) {
	client := ticketpb.NewTicketServiceClient(conn)

	req := &ticketpb.ReserveRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ReserveRequest: %w", err)
	}

	resp, err := client.Reserve(dc.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Reserve RPC failed: %w", err)
	}

	respBytes, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ReserveResponse: %w", err)
	}

	return respBytes, nil
}

// invokeTicketRelease invokes TicketService.Release
func (dc *Context) invokeTicketRelease(conn *grpc.ClientConn, reqBytes []byte) ([]byte, error) {
	client := ticketpb.NewTicketServiceClient(conn)

	req := &ticketpb.ReleaseRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ReleaseRequest: %w", err)
	}

	resp, err := client.Release(dc.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Release RPC failed: %w", err)
	}

	respBytes, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ReleaseResponse: %w", err)
	}

	return respBytes, nil
}

// getServicePort returns the port for a given service
func getServicePort(serviceName string) int {
	// Service port mapping
	ports := map[string]int{
		"HelloService":  9090,
		"TicketService": 9091,
	}

	if port, exists := ports[serviceName]; exists {
		return port
	}

	return 9000 // default port
}

// Close closes all gRPC connections
func (dc *Context) Close() {
	for _, conn := range dc.grpcConns {
		conn.Close()
	}
}

// InvocationID returns the current invocation ID
func (dc *Context) InvocationID() string {
	return dc.invocationID
}

// CurrentStep returns the current step number
func (dc *Context) CurrentStep() int32 {
	return dc.currentStep
}

// IsReplaying returns whether we're in replay mode
func (dc *Context) IsReplaying() bool {
	return dc.isReplaying
}

// SetOnStepStart sets the callback for when a step starts executing
func (dc *Context) SetOnStepStart(callback func(stepNumber int32, stepType string) error) {
	dc.onStepStart = callback
}

// SetOnStateChange sets the callback for when workflow state changes
func (dc *Context) SetOnStateChange(callback func(state map[string][]byte) error) {
	dc.onStateChange = callback
}

// SetOnDelayedCall sets the callback for when a delayed call is scheduled
func (dc *Context) SetOnDelayedCall(callback func(call *DelayedCall) error) {
	dc.onDelayedCall = callback
}

// SendDelayed schedules a delayed workflow invocation
// Similar to Restate's ctx.sendDelayed()
func (dc *Context) SendDelayed(handler string, args interface{}, delay time.Duration, partitionKey string) error {
	// Marshal args to JSON
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal delayed call args: %w", err)
	}

	// Create delayed call
	call := &DelayedCall{
		Handler:      handler,
		Args:         argsBytes,
		Delay:        delay,
		PartitionKey: partitionKey,
	}

	// Trigger callback to schedule the delayed invocation
	if dc.onDelayedCall != nil {
		if err := dc.onDelayedCall(call); err != nil {
			return fmt.Errorf("failed to schedule delayed call: %w", err)
		}
	}

	return nil
}

// Get retrieves a value from workflow-scoped state
// Similar to Restate's ctx.get()
func (dc *Context) Get(key string, value interface{}) error {
	data, exists := dc.state[key]
	if !exists {
		// Return nil error for missing keys (like Restate returns undefined/null)
		return fmt.Errorf("key not found: %s", key)
	}

	// Unmarshal JSON into the provided value
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("failed to unmarshal state for key %s: %w", key, err)
	}

	return nil
}

// Set stores a value in workflow-scoped state
// Similar to Restate's ctx.set()
func (dc *Context) Set(key string, value interface{}) error {
	// Marshal value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal state for key %s: %w", key, err)
	}

	// Update state
	dc.state[key] = data

	// Persist state if callback is set
	if dc.onStateChange != nil {
		if err := dc.onStateChange(dc.state); err != nil {
			return fmt.Errorf("failed to persist state: %w", err)
		}
	}

	return nil
}

// GetOrDefault retrieves a value from state, returning a default if not found
// Similar to Restate's (await ctx.get<T>("key")) ?? defaultValue
func (dc *Context) GetOrDefault(key string, value interface{}, defaultValue interface{}) error {
	err := dc.Get(key, value)
	if err != nil {
		// Key doesn't exist, use default
		// Need to copy defaultValue to value
		data, err := json.Marshal(defaultValue)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, value)
	}
	return nil
}

// LoadState loads workflow state from a map (called during initialization)
func (dc *Context) LoadState(stateMap map[string][]byte) {
	if stateMap != nil {
		dc.state = stateMap
	}
}

// GetState returns the current workflow state map
func (dc *Context) GetState() map[string][]byte {
	return dc.state
}

// SetOnAwakeableCreate sets the callback for when an awakeable is created
func (dc *Context) SetOnAwakeableCreate(callback func(awakeableID string) error) {
	dc.onAwakeableCreate = callback
}

// LoadAwakeables loads awakeable state from a map (called during initialization)
func (dc *Context) LoadAwakeables(awakeables map[string]*durable.AwakeableState) {
	if awakeables != nil {
		dc.awakeables = awakeables
	}
}

// GetAwakeables returns the current awakeables map
func (dc *Context) GetAwakeables() map[string]*durable.AwakeableState {
	return dc.awakeables
}

// Awakeable creates a durable awakeable that can be resolved by external systems
// Similar to Restate's ctx.awakeable()
// Returns the awakeable ID and waits for the result
func (dc *Context) Awakeable(awakeableID string) ([]byte, error) {
	stepType := fmt.Sprintf("AWAKEABLE:%s", awakeableID)

	// Notify about step start
	if dc.onStepStart != nil {
		if err := dc.onStepStart(dc.currentStep, stepType); err != nil {
			fmt.Printf("Warning: onStepStart callback failed: %v\n", err)
		}
	}

	// Check if we're replaying and this step exists in the journal
	if dc.isReplaying && int(dc.currentStep) < len(dc.journal) {
		entry := dc.journal[dc.currentStep]
		dc.currentStep++

		// Verify step type matches
		if entry.StepType != stepType {
			return nil, fmt.Errorf("step type mismatch: expected %s, got %s", stepType, entry.StepType)
		}

		// Check if the step errored (awakeable was rejected)
		if entry.Error != "" {
			return nil, fmt.Errorf("awakeable rejected: %s", entry.Error)
		}

		// Return cached result
		return entry.Response, nil
	}

	// We're past replay or this is a new execution
	dc.isReplaying = false

	// Check if awakeable already exists
	if awakeable, exists := dc.awakeables[awakeableID]; exists {
		if awakeable.Resolved {
			// Awakeable was resolved, journal and return result
			entry := &durable.JournalEntry{
				StepNumber: dc.currentStep,
				StepType:   stepType,
				Response:   awakeable.Result,
				Error:      awakeable.Error,
			}

			if dc.onJournalStep != nil {
				if err := dc.onJournalStep(entry); err != nil {
					return nil, fmt.Errorf("failed to journal awakeable result: %w", err)
				}
			}

			dc.currentStep++

			if awakeable.Error != "" {
				return nil, fmt.Errorf("awakeable rejected: %s", awakeable.Error)
			}

			return awakeable.Result, nil
		}
	} else {
		// Create new awakeable
		awakeable := &durable.AwakeableState{
			AwakeableId:  awakeableID,
			InvocationId: dc.invocationID,
			Resolved:     false,
			CreatedAt:    time.Now().Unix(),
		}

		dc.awakeables[awakeableID] = awakeable

		// Notify callback (processor will persist this)
		if dc.onAwakeableCreate != nil {
			if err := dc.onAwakeableCreate(awakeableID); err != nil {
				return nil, fmt.Errorf("failed to create awakeable: %w", err)
			}
		}
	}

	// Awakeable not resolved yet - suspend workflow
	return nil, &AwakeableSuspendError{AwakeableID: awakeableID}
}
