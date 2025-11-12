package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"
	"google.golang.org/grpc"
)

// HelloServiceImpl implements the HelloService gRPC service
type HelloServiceImpl struct {
	hellopb.UnimplementedHelloServiceServer
}

// SayHello implements the SayHello RPC method
func (s *HelloServiceImpl) SayHello(ctx context.Context, req *hellopb.HelloRequest) (*hellopb.HelloResponse, error) {
	log.Printf("Received SayHello request: name=%s", req.Name)

	message := fmt.Sprintf("Hello, %s! Welcome to Durable Execution on NATS JetStream.", req.Name)

	return &hellopb.HelloResponse{
		Message: message,
	}, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting HelloService gRPC server...")

	// Get port from environment or use default
	port := getEnv("GRPC_PORT", "9090")
	addr := fmt.Sprintf(":%s", port)

	// Create TCP listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register HelloService
	helloService := &HelloServiceImpl{}
	hellopb.RegisterHelloServiceServer(grpcServer, helloService)

	log.Printf("HelloService listening on %s", addr)

	// Start server in goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gRPC server...")
	grpcServer.GracefulStop()
	log.Println("Server stopped")
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
