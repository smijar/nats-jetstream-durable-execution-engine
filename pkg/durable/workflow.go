package durable

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// Workflow represents a type-safe durable workflow
type Workflow[TIn, TOut any] struct {
	name    string
	handler func(*Context, TIn) (TOut, error)
}

// NewWorkflow creates a new type-safe workflow definition
func NewWorkflow[TIn, TOut any](name string, handler func(*Context, TIn) (TOut, error)) *Workflow[TIn, TOut] {
	return &Workflow[TIn, TOut]{
		name:    name,
		handler: handler,
	}
}

// Name returns the workflow name
func (w *Workflow[TIn, TOut]) Name() string {
	return w.name
}

// Handler returns the underlying handler function
func (w *Workflow[TIn, TOut]) Handler() func(*Context, TIn) (TOut, error) {
	return w.handler
}

// Ref returns a workflow reference for remote invocation (without handler code)
func (w *Workflow[TIn, TOut]) Ref() *WorkflowRef[TIn, TOut] {
	return &WorkflowRef[TIn, TOut]{
		name: w.name,
	}
}

// Execute runs the workflow with the given input
func (w *Workflow[TIn, TOut]) Execute(ctx *Context, input TIn) (TOut, error) {
	return w.handler(ctx, input)
}

// adaptToHandler adapts this typed workflow to an untyped handler
func (w *Workflow[TIn, TOut]) adaptToHandler() func(*Context) error {
	return workflowAdapter(w)
}

// WorkflowRef is a reference to a workflow for remote invocation
type WorkflowRef[TIn, TOut any] struct {
	name string
}

// Name returns the workflow name
func (w *WorkflowRef[TIn, TOut]) Name() string {
	return w.name
}

// workflowAdapter adapts a typed workflow to the untyped HandlerFunc
func workflowAdapter[TIn, TOut any](workflow *Workflow[TIn, TOut]) func(*Context) error {
	return func(ctx *Context) error {
		// Get input from context (stored as JSON in Args field)
		var input TIn
		if len(ctx.args) > 0 {
			if err := unmarshalInput(ctx.args, &input); err != nil {
				return fmt.Errorf("failed to unmarshal input: %w", err)
			}
		}

		// Execute workflow
		output, err := workflow.handler(ctx, input)
		if err != nil {
			return err
		}

		// Store output in context for later retrieval
		outputBytes, err := marshalOutput(output)
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}
		ctx.output = outputBytes

		return nil
	}
}

// unmarshalInput unmarshals input from bytes
func unmarshalInput[T any](data []byte, input *T) error {
	// Try proto first, then JSON
	if msg, ok := any(input).(proto.Message); ok {
		return proto.Unmarshal(data, msg)
	}
	return json.Unmarshal(data, input)
}

// marshalOutput marshals output to bytes
func marshalOutput[T any](output T) ([]byte, error) {
	// Try proto first, then JSON
	if msg, ok := any(output).(proto.Message); ok {
		return proto.Marshal(msg)
	}
	return json.Marshal(output)
}

// WorkflowRegistry manages registered workflows
type WorkflowRegistry struct {
	handlers map[string]func(*Context) error
}

// NewWorkflowRegistry creates a new workflow registry
func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		handlers: make(map[string]func(*Context) error),
	}
}

// Register registers a typed workflow using interface{} for flexibility
func (r *WorkflowRegistry) Register(workflow interface{}) {
	// Use type assertion to extract the workflow
	// This is a workaround for Go's lack of generic methods on non-generic types
	switch w := workflow.(type) {
	case interface{ Name() string; adaptToHandler() func(*Context) error }:
		r.handlers[w.Name()] = w.adaptToHandler()
	default:
		panic(fmt.Sprintf("unsupported workflow type: %T", workflow))
	}
}

// Get retrieves a handler by name
func (r *WorkflowRegistry) Get(name string) (func(*Context) error, bool) {
	handler, exists := r.handlers[name]
	return handler, exists
}

// GetAll returns all registered handlers
func (r *WorkflowRegistry) GetAll() map[string]func(*Context) error {
	return r.handlers
}

// RegisterLegacy registers a legacy-style handler
func (r *WorkflowRegistry) RegisterLegacy(name string, handler func(*Context) error) {
	r.handlers[name] = handler
}

// SetArgs sets the workflow input arguments
func (c *Context) SetArgs(args []byte) {
	c.args = args
}

// GetOutput returns the workflow output result
func (c *Context) GetOutput() []byte {
	return c.output
}
