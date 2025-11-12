package main

import (
	"context"
	"fmt"
	"log"
	"time"

	hellopb "github.com/sanjaymijar/my-durable-execution/pb/hello"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	serviceAddress = "localhost:9090"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("========================================")
	fmt.Println("Direct gRPC Service Call Example")
	fmt.Println("========================================")
	fmt.Println()

	log.Printf("Attempting to connect to HelloService at %s...", serviceAddress)

	// Set up a connection to the gRPC server.
	conn, err := grpc.Dial(serviceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	log.Println("✅ Connected to gRPC server.")

	// Create a client for the HelloService.
	c := hellopb.NewHelloServiceClient(conn)

	// Set a timeout for the RPC call.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create the request payload.
	req := &hellopb.HelloRequest{Name: "Direct Client"}

	log.Printf("📞 Making SayHello RPC call with name: \"%s\"", req.Name)

	// Make the RPC call.
	res, err := c.SayHello(ctx, req)
	if err != nil {
		log.Fatalf("❌ Could not greet: %v", err)
	}

	log.Printf("✅ Received response: %s", res.GetMessage())
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("✓ Direct gRPC call demo complete!")
	fmt.Println("========================================")
}
