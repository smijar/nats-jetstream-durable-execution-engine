package main

import (
	"context"
	"log"
	"net"
	"sync"

	ticketpb "github.com/sanjaymijar/my-durable-execution/pb/ticket"
	"google.golang.org/grpc"
)

// TicketServiceImpl implements the TicketService
type TicketServiceImpl struct {
	ticketpb.UnimplementedTicketServiceServer
	mu               sync.Mutex
	reservedTickets  map[string]bool // ticketID -> reserved
	availableTickets []string
}

func NewTicketService() *TicketServiceImpl {
	// Initialize with some available tickets
	return &TicketServiceImpl{
		reservedTickets: make(map[string]bool),
		availableTickets: []string{
			"TICKET-001", "TICKET-002", "TICKET-003",
			"TICKET-004", "TICKET-005", "TICKET-006",
			"TICKET-007", "TICKET-008", "TICKET-009",
			"TICKET-010",
		},
	}
}

func (s *TicketServiceImpl) Reserve(ctx context.Context, req *ticketpb.ReserveRequest) (*ticketpb.ReserveResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("🎫 Reserve request for ticket: %s", req.TicketId)

	// Check if ticket exists
	found := false
	for _, ticket := range s.availableTickets {
		if ticket == req.TicketId {
			found = true
			break
		}
	}

	if !found {
		log.Printf("❌ Ticket not found: %s", req.TicketId)
		return &ticketpb.ReserveResponse{
			Success: false,
			Message: "Ticket not found",
		}, nil
	}

	// Check if already reserved
	if s.reservedTickets[req.TicketId] {
		log.Printf("❌ Ticket already reserved: %s", req.TicketId)
		return &ticketpb.ReserveResponse{
			Success: false,
			Message: "Ticket already reserved",
		}, nil
	}

	// Reserve the ticket
	s.reservedTickets[req.TicketId] = true
	log.Printf("✅ Ticket reserved successfully: %s", req.TicketId)

	return &ticketpb.ReserveResponse{
		Success: true,
		Message: "Ticket reserved successfully",
	}, nil
}

func (s *TicketServiceImpl) Release(ctx context.Context, req *ticketpb.ReleaseRequest) (*ticketpb.ReleaseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("🔓 Release request for ticket: %s", req.TicketId)

	if !s.reservedTickets[req.TicketId] {
		log.Printf("⚠️  Ticket was not reserved: %s", req.TicketId)
		return &ticketpb.ReleaseResponse{Success: false}, nil
	}

	delete(s.reservedTickets, req.TicketId)
	log.Printf("✅ Ticket released: %s", req.TicketId)

	return &ticketpb.ReleaseResponse{Success: true}, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting TicketService on port 9091...")

	lis, err := net.Listen("tcp", ":9091")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	ticketService := NewTicketService()
	ticketpb.RegisterTicketServiceServer(grpcServer, ticketService)

	log.Println("✅ TicketService ready on port 9091")
	log.Println("   Available tickets: TICKET-001 through TICKET-010")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
