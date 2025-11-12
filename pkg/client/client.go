package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/sanjaymijar/my-durable-execution/pb/durable"
	durableSDK "github.com/sanjaymijar/my-durable-execution/pkg/durable"
	"github.com/sanjaymijar/my-durable-execution/pkg/execution"
	"github.com/sanjaymijar/my-durable-execution/pkg/jetstream"
	"google.golang.org/protobuf/proto"
)

// Client provides a simple high-level API for durable execution
type Client struct {
	jsClient  *jetstream.Client
	processor *execution.Processor
	timeout   time.Duration
	registry  *durableSDK.WorkflowRegistry
	serving   bool
}

// NewClient creates a new durable execution client
func NewClient(natsURL string) (*Client, error) {
	return NewClientWithTimeout(natsURL, 30*time.Second)
}

// NewClientWithTimeout creates a new client with custom timeout
func NewClientWithTimeout(natsURL string, timeout time.Duration) (*Client, error) {
	jsClient, err := jetstream.NewClient(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	processor, err := execution.NewProcessor(jsClient)
	if err != nil {
		jsClient.Close()
		return nil, fmt.Errorf("failed to create processor: %w", err)
	}

	return &Client{
		jsClient:  jsClient,
		processor: processor,
		timeout:   timeout,
		registry:  durableSDK.NewWorkflowRegistry(),
		serving:   false,
	}, nil
}

// Invoke executes a workflow and waits for the result
func (c *Client) Invoke(ctx context.Context, handler string, args ...InvokeOption) (*ExecutionResult, error) {
	opts := &invokeOpts{
		partitionKey: "default",
		service:      "HelloService",
	}

	for _, opt := range args {
		opt(opts)
	}

	// Create command
	cmd := &durable.Command{
		InvocationId: uuid.New().String(),
		Handler:      handler,
		Service:      opts.service,
		Args:         opts.args,
		PartitionKey: opts.partitionKey,
		Sequence:     1,
	}

	// Submit command
	if err := c.processor.SubmitCommand(ctx, cmd); err != nil {
		return nil, fmt.Errorf("failed to submit command: %w", err)
	}

	// Wait for completion
	state, err := c.waitForCompletion(ctx, cmd.InvocationId)
	if err != nil {
		return nil, err
	}

	return &ExecutionResult{
		InvocationID:    state.InvocationId,
		Handler:         state.Handler,
		Status:          state.Status,
		Journal:         state.Journal,
		CurrentState:    state.CurrentState,
		CurrentStep:     state.CurrentStep,
		CurrentStepType: state.CurrentStepType,
	}, nil
}

// InvokeAsync submits a workflow without waiting for the result
func (c *Client) InvokeAsync(ctx context.Context, handler string, args ...InvokeOption) (string, error) {
	opts := &invokeOpts{
		partitionKey: "default",
		service:      "HelloService",
	}

	for _, opt := range args {
		opt(opts)
	}

	cmd := &durable.Command{
		InvocationId: uuid.New().String(),
		Handler:      handler,
		Service:      opts.service,
		Args:         opts.args,
		PartitionKey: opts.partitionKey,
		Sequence:     1,
	}

	if err := c.processor.SubmitCommand(ctx, cmd); err != nil {
		return "", fmt.Errorf("failed to submit command: %w", err)
	}

	return cmd.InvocationId, nil
}

// GetResult retrieves the result of a workflow execution
func (c *Client) GetResult(ctx context.Context, invocationID string) (*ExecutionResult, error) {
	kv, err := c.jsClient.GetStateKV(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket: %w", err)
	}

	entry, err := kv.Get(ctx, invocationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	var state durable.ExecutionState
	if err := proto.Unmarshal(entry.Value(), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &ExecutionResult{
		InvocationID:    state.InvocationId,
		Handler:         state.Handler,
		Status:          state.Status,
		Journal:         state.Journal,
		CurrentState:    state.CurrentState,
		CurrentStep:     state.CurrentStep,
		CurrentStepType: state.CurrentStepType,
	}, nil
}

// Cancel cancels a running workflow execution
func (c *Client) Cancel(ctx context.Context, invocationID string) error {
	return c.updateWorkflowStatus(ctx, invocationID, "cancelled")
}

// Pause pauses a running workflow execution (can be resumed later)
func (c *Client) Pause(ctx context.Context, invocationID string) error {
	return c.updateWorkflowStatus(ctx, invocationID, "paused")
}

// Resume resumes a paused workflow execution (same invocation ID continues)
func (c *Client) Resume(ctx context.Context, invocationID string) error {
	return c.updateWorkflowStatus(ctx, invocationID, "pending")
}

// ====================================================================================
// Query API - Workflow introspection and monitoring
// ====================================================================================

// GetWorkflow retrieves detailed information about a specific workflow
// Alias for GetResult for API consistency
func (c *Client) GetWorkflow(ctx context.Context, invocationID string) (*WorkflowDetails, error) {
	result, err := c.GetResult(ctx, invocationID)
	if err != nil {
		return nil, err
	}

	return &WorkflowDetails{
		InvocationID:    result.InvocationID,
		Handler:         result.Handler,
		Status:          result.Status,
		CurrentStep:     result.CurrentStep,
		CurrentStepType: result.CurrentStepType,
		TotalSteps:      int32(len(result.Journal)),
		Journal:         result.Journal,
		Output:          result.CurrentState,
	}, nil
}

// ListWorkflows lists all workflows with optional filtering
func (c *Client) ListWorkflows(ctx context.Context, filters ...WorkflowFilter) (*WorkflowList, error) {
	kv, err := c.jsClient.GetStateKV(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket: %w", err)
	}

	// Get all keys from KV bucket
	keys, err := kv.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	// Build filter options
	opts := &workflowFilterOpts{}
	for _, f := range filters {
		f(opts)
	}

	workflows := []WorkflowSummary{}

	for _, keyName := range keys {
		entry, err := kv.Get(ctx, keyName)
		if err != nil {
			continue // Skip errors
		}

		var state durable.ExecutionState
		if err := proto.Unmarshal(entry.Value(), &state); err != nil {
			continue // Skip malformed entries
		}

		// Apply filters
		if opts.status != "" && state.Status != opts.status {
			continue
		}
		if opts.handler != "" && state.Handler != opts.handler {
			continue
		}

		workflows = append(workflows, WorkflowSummary{
			InvocationID:    state.InvocationId,
			Handler:         state.Handler,
			Status:          state.Status,
			TotalSteps:      int32(len(state.Journal)),
			CurrentStep:     state.CurrentStep,
			CurrentStepType: state.CurrentStepType,
		})

		// Apply limit
		if opts.limit > 0 && len(workflows) >= opts.limit {
			break
		}
	}

	return &WorkflowList{
		Workflows: workflows,
		Total:     len(workflows),
	}, nil
}

// GetWorkflowStats returns statistics about workflows
func (c *Client) GetWorkflowStats(ctx context.Context) (*WorkflowStats, error) {
	kv, err := c.jsClient.GetStateKV(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket: %w", err)
	}

	keys, err := kv.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	stats := &WorkflowStats{
		ByStatus:  make(map[string]int),
		ByHandler: make(map[string]int),
	}

	for _, keyName := range keys {
		entry, err := kv.Get(ctx, keyName)
		if err != nil {
			continue
		}

		var state durable.ExecutionState
		if err := proto.Unmarshal(entry.Value(), &state); err != nil {
			continue
		}

		stats.Total++
		stats.ByStatus[state.Status]++
		stats.ByHandler[state.Handler]++
	}

	return stats, nil
}

// updateWorkflowStatus updates the status of a workflow in the KV store
func (c *Client) updateWorkflowStatus(ctx context.Context, invocationID string, newStatus string) error {
	kv, err := c.jsClient.GetStateKV(ctx)
	if err != nil {
		return fmt.Errorf("failed to get KV bucket: %w", err)
	}

	// Get current state
	entry, err := kv.Get(ctx, invocationID)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	var state durable.ExecutionState
	if err := proto.Unmarshal(entry.Value(), &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Update status
	state.Status = newStatus

	// Save back to KV
	stateBytes, err := proto.Marshal(&state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if _, err := kv.Put(ctx, invocationID, stateBytes); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	return nil
}

// Close closes the client connection
func (c *Client) Close() {
	if c.jsClient != nil {
		c.jsClient.Close()
	}
}

// waitForCompletion polls until execution completes
func (c *Client) waitForCompletion(ctx context.Context, invocationID string) (*durable.ExecutionState, error) {
	kv, err := c.jsClient.GetStateKV(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket: %w", err)
	}

	deadline := time.Now().Add(c.timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for execution")
			}

			entry, err := kv.Get(ctx, invocationID)
			if err != nil {
				if err == natsjs.ErrKeyNotFound {
					continue
				}
				return nil, fmt.Errorf("failed to get state: %w", err)
			}

			var state durable.ExecutionState
			if err := proto.Unmarshal(entry.Value(), &state); err != nil {
				return nil, fmt.Errorf("failed to unmarshal state: %w", err)
			}

			if state.Status == "completed" {
				return &state, nil
			}
			if state.Status == "failed" {
				return &state, fmt.Errorf("execution failed")
			}
		}
	}
}

// ExecutionResult represents the result of a workflow execution
type ExecutionResult struct {
	InvocationID    string
	Handler         string
	Status          string
	Journal         []*durable.JournalEntry
	CurrentState    []byte // Workflow output
	CurrentStep     int32  // Current step number being executed (-1 if not started/completed)
	CurrentStepType string // Type of current step (e.g., "HelloService.SayHello")
}

// GetResponse extracts and unmarshals the response from a specific step
func (r *ExecutionResult) GetResponse(stepNumber int32, resp proto.Message) error {
	for _, entry := range r.Journal {
		if entry.StepNumber == stepNumber {
			if entry.Error != "" {
				return fmt.Errorf("step failed: %s", entry.Error)
			}
			return proto.Unmarshal(entry.Response, resp)
		}
	}
	return fmt.Errorf("step %d not found", stepNumber)
}

// GetFirstResponse gets the response from the first step
func (r *ExecutionResult) GetFirstResponse(resp proto.Message) error {
	return r.GetResponse(0, resp)
}

// InvokeOption configures workflow invocation
type InvokeOption func(*invokeOpts)

type invokeOpts struct {
	partitionKey string
	service      string
	args         []byte
}

// WithPartitionKey sets the partition key for the workflow
func WithPartitionKey(key string) InvokeOption {
	return func(o *invokeOpts) {
		o.partitionKey = key
	}
}

// WithService sets the target service
func WithService(service string) InvokeOption {
	return func(o *invokeOpts) {
		o.service = service
	}
}

// WithArgs sets the workflow arguments
func WithArgs(args []byte) InvokeOption {
	return func(o *invokeOpts) {
		o.args = args
	}
}

// ====================================================================================
// Query API Types
// ====================================================================================

// WorkflowDetails provides detailed information about a workflow
type WorkflowDetails struct {
	InvocationID    string
	Handler         string
	Status          string
	CurrentStep     int32
	CurrentStepType string
	TotalSteps      int32
	Journal         []*durable.JournalEntry
	Output          []byte
}

// WorkflowSummary provides summary information about a workflow
type WorkflowSummary struct {
	InvocationID    string
	Handler         string
	Status          string
	TotalSteps      int32
	CurrentStep     int32
	CurrentStepType string
}

// WorkflowList contains a list of workflows
type WorkflowList struct {
	Workflows []WorkflowSummary
	Total     int
}

// WorkflowStats provides statistics about workflows
type WorkflowStats struct {
	Total     int
	ByStatus  map[string]int
	ByHandler map[string]int
}

// WorkflowFilter configures workflow list filtering
type WorkflowFilter func(*workflowFilterOpts)

type workflowFilterOpts struct {
	status  string
	handler string
	limit   int
}

// WithStatus filters workflows by status
func WithStatus(status string) WorkflowFilter {
	return func(o *workflowFilterOpts) {
		o.status = status
	}
}

// WithHandler filters workflows by handler name
func WithHandler(handler string) WorkflowFilter {
	return func(o *workflowFilterOpts) {
		o.handler = handler
	}
}

// WithLimit limits the number of results
func WithLimit(limit int) WorkflowFilter {
	return func(o *workflowFilterOpts) {
		o.limit = limit
	}
}

// ====================================================================================


// ====================================================================================
// Type-Safe Workflow API (Restate-style)
// ====================================================================================

// Register registers a typed workflow for local execution
func (c *Client) Register(workflow interface{}) {
	c.registry.Register(workflow)
}

// ServeHandlers starts serving registered handlers locally
func (c *Client) ServeHandlers(ctx context.Context) error {
	c.serving = true
	
	log.Println("Serving workflows locally...")
	
	<-ctx.Done()
	return ctx.Err()
}

// executeLocal executes a workflow locally without going through NATS
func (c *Client) executeLocal(ctx context.Context, workflowName string, inputBytes []byte) ([]byte, error) {
	handler, exists := c.registry.Get(workflowName)
	if !exists {
		return nil, fmt.Errorf("workflow not registered: %s", workflowName)
	}

	// Create a durable context for local execution
	durableCtx := durableSDK.NewContext(ctx, uuid.New().String(), nil, func(entry *durable.JournalEntry) error {
		// In local mode, we don't persist journal entries
		return nil
	})
	durableCtx.SetArgs(inputBytes)

	// Execute the handler
	if err := handler(durableCtx); err != nil {
		return nil, err
	}

	// Return the output
	return durableCtx.GetOutput(), nil
}

// Helper functions for marshaling/unmarshaling
func marshalInput[T any](input T) ([]byte, error) {
	if msg, ok := any(input).(proto.Message); ok {
		return proto.Marshal(msg)
	}
	// For simple types, use JSON
	return json.Marshal(input)
}

func unmarshalOutput[T any](data []byte, output *T) error {
	if msg, ok := any(output).(proto.Message); ok {
		return proto.Unmarshal(data, msg)
	}
	// For simple types, use JSON
	return json.Unmarshal(data, output)
}

// ====================================================================================
// Standalone Generic Functions (workaround for lack of generic methods)
// ====================================================================================

// InvokeWorkflow invokes a typed workflow with type-safe input and output
func InvokeWorkflow[TIn, TOut any](c *Client, ctx context.Context, workflow interface{}, input TIn) (TOut, error) {
	var zero TOut
	
	// Get workflow name
	var workflowName string
	switch w := workflow.(type) {
	case *durableSDK.Workflow[TIn, TOut]:
		workflowName = w.Name()
	case *durableSDK.WorkflowRef[TIn, TOut]:
		workflowName = w.Name()
	default:
		return zero, fmt.Errorf("invalid workflow type")
	}

	// Marshal input
	inputBytes, err := marshalInput(input)
	if err != nil {
		return zero, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Check if we're serving locally
	if c.serving {
		// Execute locally
		result, err := c.executeLocal(ctx, workflowName, inputBytes)
		if err != nil {
			return zero, err
		}
		
		// Unmarshal output
		var output TOut
		if err := unmarshalOutput(result, &output); err != nil {
			return zero, fmt.Errorf("failed to unmarshal output: %w", err)
		}
		return output, nil
	}

	// Execute remotely via NATS
	result, err := c.Invoke(ctx, workflowName, WithArgs(inputBytes))
	if err != nil {
		return zero, err
	}

	// Extract output from workflow's CurrentState (where workflow output is stored)
	var output TOut
	if len(result.CurrentState) > 0 {
		if err := unmarshalOutput(result.CurrentState, &output); err != nil {
			return zero, fmt.Errorf("failed to unmarshal output: %w", err)
		}
	}

	return output, nil
}
