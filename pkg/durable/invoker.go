package durable

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	ticketpb "github.com/sanjaymijar/my-durable-execution/pb/ticket"
)

// InvokerType defines the type of invoker (e.g., grpc, http)
type InvokerType string

const (
	GRPC InvokerType = "grpc"
	HTTP InvokerType = "http"
)

// ServiceCall represents a generic service call
type ServiceCall struct {
	ServiceName string
	MethodName  string
	Request     []byte
}

// Invoker is the interface for any service communication protocol
type Invoker interface {
	Invoke(ctx context.Context, call ServiceCall) ([]byte, error)
	Close() error
}

// GRPCInvoker handles gRPC service invocations
type GRPCInvoker struct {
	address string
	conn    *grpc.ClientConn
}

// NewGRPCInvoker creates a new GRPCInvoker
func NewGRPCInvoker(address string) *GRPCInvoker {
	return &GRPCInvoker{
		address: address,
	}
}

// getOrCreateConnection gets or creates a gRPC connection
func (i *GRPCInvoker) getOrCreateConnection() (*grpc.ClientConn, error) {
	if i.conn != nil {
		return i.conn, nil
	}

	conn, err := grpc.Dial(i.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC service at %s: %w", i.address, err)
	}
	i.conn = conn
	return i.conn, nil
}

// Invoke executes the gRPC call
func (i *GRPCInvoker) Invoke(ctx context.Context, call ServiceCall) ([]byte, error) {
	conn, err := i.getOrCreateConnection()
	if err != nil {
		return nil, err
	}

	// This is now contained within the GRPCInvoker.
	// A more advanced solution could use gRPC reflection or a registration mechanism
	// for method handlers to make this more generic.
	switch call.ServiceName {
	case "HelloService":
		return i.invokeHelloService(ctx, conn, call.MethodName, call.Request)
	case "TicketService":
		return i.invokeTicketService(ctx, conn, call.MethodName, call.Request)
	default:
		return nil, fmt.Errorf("unsupported gRPC service: %s", call.ServiceName)
	}
}

func (i *GRPCInvoker) invokeHelloService(ctx context.Context, conn *grpc.ClientConn, method string, reqBytes []byte) ([]byte, error) {
	client := hellopb.NewHelloServiceClient(conn)

	switch method {
	case "SayHello":
		req := &hellopb.HelloRequest{}
		if err := proto.Unmarshal(reqBytes, req); err != nil {
			return nil, fmt.Errorf("failed to unmarshal HelloRequest: %w", err)
		}
		resp, err := client.SayHello(ctx, req)
		if err != nil {
			return nil, err
		}
		return proto.Marshal(resp)
	default:
		return nil, fmt.Errorf("unsupported method for HelloService: %s", method)
	}
}

func (i *GRPCInvoker) invokeTicketService(ctx context.Context, conn *grpc.ClientConn, method string, reqBytes []byte) ([]byte, error) {
	client := ticketpb.NewTicketServiceClient(conn)

	switch method {
	case "Reserve":
		req := &ticketpb.ReserveRequest{}
		if err := proto.Unmarshal(reqBytes, req); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ReserveRequest: %w", err)
		}
		resp, err := client.Reserve(ctx, req)
		if err != nil {
			return nil, err
		}
		return proto.Marshal(resp)
	case "Release":
		req := &ticketpb.ReleaseRequest{}
		if err := proto.Unmarshal(reqBytes, req); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ReleaseRequest: %w", err)
		}
		resp, err := client.Release(ctx, req)
		if err != nil {
			return nil, err
		}
		return proto.Marshal(resp)
	default:
		return nil, fmt.Errorf("unsupported method for TicketService: %s", method)
	}
}

// Close closes the gRPC connection
func (i *GRPCInvoker) Close() error {
	if i.conn != nil {
		return i.conn.Close()
	}
	return nil
}

// HTTPInvoker is a placeholder for a future HTTP implementation
type HTTPInvoker struct {
	address string
}

// NewHTTPInvoker creates a new HTTPInvoker
func NewHTTPInvoker(address string) *HTTPInvoker {
	return &HTTPInvoker{address: address}
}

// Invoke for HTTPInvoker (not implemented)
func (i *HTTPInvoker) Invoke(ctx context.Context, call ServiceCall) ([]byte, error) {
	// TODO: Implement HTTP calls, e.g., using net/http.Client
	return nil, fmt.Errorf("HTTPInvoker not implemented")
}

// Close for HTTPInvoker
func (i *HTTPInvoker) Close() error {
	return nil
}
