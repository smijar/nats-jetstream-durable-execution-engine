package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/sanjaymijar/my-durable-execution/pb/durable"
	"github.com/sanjaymijar/my-durable-execution/pkg/client"
)

// ProcessorInterface defines methods needed from processor
type ProcessorInterface interface {
	ResolveAwakeable(ctx context.Context, awakeableID string, result []byte, errorMsg string) error
}

// Server provides HTTP API for querying workflows
type Server struct {
	client    *client.Client
	processor ProcessorInterface
	server    *http.Server
}

// NewServer creates a new API server
func NewServer(port int, c *client.Client, processor ProcessorInterface) *Server {
	mux := http.NewServeMux()

	s := &Server{
		client:    c,
		processor: processor,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}

	// Register routes
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/workflows", s.handleListWorkflows)
	mux.HandleFunc("/api/workflows/stats", s.handleWorkflowStats)
	mux.HandleFunc("/api/workflows/", s.handleGetWorkflow) // Note: trailing slash for pattern matching
	mux.HandleFunc("/api/awakeables/", s.handleResolveAwakeable)

	return s
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	log.Printf("Starting HTTP API server on %s", s.server.Addr)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		log.Println("Shutting down HTTP API server...")
		return s.server.Shutdown(context.Background())
	case err := <-errChan:
		return err
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// handleGetWorkflow handles GET /api/workflows/{invocationID}
func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract invocation ID from URL path
	// Path format: /api/workflows/{invocationID}
	path := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	if path == "" || path == "stats" {
		http.Error(w, "Invocation ID required", http.StatusBadRequest)
		return
	}

	invocationID := path

	// Get workflow details
	details, err := s.client.GetWorkflow(r.Context(), invocationID)
	if err != nil {
		log.Printf("Error getting workflow %s: %v", invocationID, err)
		http.Error(w, fmt.Sprintf("Failed to get workflow: %v", err), http.StatusNotFound)
		return
	}

	// Build response
	response := map[string]interface{}{
		"invocation_id":     details.InvocationID,
		"handler":           details.Handler,
		"status":            details.Status,
		"current_step":      details.CurrentStep,
		"current_step_type": details.CurrentStepType,
		"total_steps":       details.TotalSteps,
		"journal":           buildJournalResponse(details.Journal),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleListWorkflows handles GET /api/workflows?status=X&handler=Y&limit=N
func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	status := query.Get("status")
	handler := query.Get("handler")
	limit := 0
	if limitStr := query.Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	// Build filters
	filters := []client.WorkflowFilter{}
	if status != "" {
		filters = append(filters, client.WithStatus(status))
	}
	if handler != "" {
		filters = append(filters, client.WithHandler(handler))
	}
	if limit > 0 {
		filters = append(filters, client.WithLimit(limit))
	}

	// List workflows
	list, err := s.client.ListWorkflows(r.Context(), filters...)
	if err != nil {
		log.Printf("Error listing workflows: %v", err)
		http.Error(w, fmt.Sprintf("Failed to list workflows: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	workflows := make([]map[string]interface{}, len(list.Workflows))
	for i, wf := range list.Workflows {
		workflows[i] = map[string]interface{}{
			"invocation_id":     wf.InvocationID,
			"handler":           wf.Handler,
			"status":            wf.Status,
			"total_steps":       wf.TotalSteps,
			"current_step":      wf.CurrentStep,
			"current_step_type": wf.CurrentStepType,
		}
	}

	response := map[string]interface{}{
		"workflows": workflows,
		"total":     list.Total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleWorkflowStats handles GET /api/workflows/stats
func (s *Server) handleWorkflowStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get stats
	stats, err := s.client.GetWorkflowStats(r.Context())
	if err != nil {
		log.Printf("Error getting workflow stats: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	response := map[string]interface{}{
		"total":      stats.Total,
		"by_status":  stats.ByStatus,
		"by_handler": stats.ByHandler,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// buildJournalResponse converts journal entries to JSON-friendly format
func buildJournalResponse(journal []*durable.JournalEntry) []map[string]interface{} {
	result := make([]map[string]interface{}, len(journal))
	for i, entry := range journal {
		result[i] = map[string]interface{}{
			"step_number": entry.StepNumber,
			"step_type":   entry.StepType,
			"error":       entry.Error,
			// Note: request/response bytes are omitted for brevity
			// Can be added if needed for debugging
		}
	}
	return result
}

// handleResolveAwakeable handles POST /api/awakeables/{awakeableID}/resolve
// or POST /api/awakeables/{awakeableID}/reject
func (s *Server) handleResolveAwakeable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract awakeable ID from URL path
	// Path format: /api/awakeables/{awakeableID}/resolve or /api/awakeables/{awakeableID}/reject
	path := strings.TrimPrefix(r.URL.Path, "/api/awakeables/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path format. Expected: /api/awakeables/{awakeableID}/resolve", http.StatusBadRequest)
		return
	}

	awakeableID := parts[0]
	action := parts[1]

	if action != "resolve" && action != "reject" {
		http.Error(w, "Action must be 'resolve' or 'reject'", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req struct {
		Result string `json:"result"` // JSON string result
		Error  string `json:"error"`  // Error message (for reject)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Resolve the awakeable
	var result []byte
	var errorMsg string

	if action == "reject" {
		errorMsg = req.Error
		if errorMsg == "" {
			errorMsg = "Awakeable rejected"
		}
	} else {
		result = []byte(req.Result)
	}

	if err := s.processor.ResolveAwakeable(r.Context(), awakeableID, result, errorMsg); err != nil {
		log.Printf("Error resolving awakeable %s: %v", awakeableID, err)
		http.Error(w, fmt.Sprintf("Failed to resolve awakeable: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"awakeable_id":  awakeableID,
		"action":        action,
	})
}
