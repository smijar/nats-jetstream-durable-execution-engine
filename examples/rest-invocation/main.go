package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	baseURL      = "http://localhost:8080/api"
	workflowName = "hello_workflow"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("========================================")
	fmt.Println("REST Invocation Example (Go Client with Resty)")
	fmt.Println("========================================")
	fmt.Println()

	client := resty.New()
	client.SetTimeout(10 * time.Second)

	// 1. Invoke the workflow via REST API
	fmt.Println("📤 Step 1: Invoking workflow via HTTP POST...")
	workflowInput := map[string]string{"name": "Go Resty Client"}
	invocationID, err := invokeWorkflow(client, workflowInput)
	if err != nil {
		log.Fatalf("❌ Failed to invoke workflow: %v", err)
	}
	fmt.Printf("✅ Workflow submitted successfully! Invocation ID: %s\n", invocationID)
	fmt.Println()

	// 2. Wait for the workflow to complete
	fmt.Println("⏳ Step 2: Waiting 2 seconds for workflow to complete...")
	time.Sleep(2 * time.Second)
	fmt.Println()

	// 3. Check the workflow status via REST API
	fmt.Println("🔍 Step 3: Checking workflow status via HTTP GET...")
	status, err := getWorkflowStatus(client, invocationID)
	if err != nil {
		log.Fatalf("❌ Failed to get workflow status: %v", err)
	}
	fmt.Printf("✅ Successfully retrieved status:\n")
	prettyStatus, _ := json.MarshalIndent(status, "", "  ")
	fmt.Println(string(prettyStatus))
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("✓ REST invocation demo complete!")
	fmt.Println("========================================")
}

// invokeWorkflow starts a workflow and returns its invocation ID.
func invokeWorkflow(client *resty.Client, input map[string]string) (string, error) {
	url := fmt.Sprintf("%s/workflows/%s/invoke", baseURL, workflowName)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(input).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}

	// Check for successful status code
	if resp.StatusCode() != http.StatusAccepted {
		return "", fmt.Errorf("received non-202 status code: %d - %s", resp.StatusCode(), resp.String())
	}

	// Decode the response to get the invocation ID
	var result struct {
		InvocationID string `json:"invocation_id"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	return result.InvocationID, nil
}

// getWorkflowStatus retrieves the status of a specific workflow execution.
func getWorkflowStatus(client *resty.Client, invocationID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/workflows/%s", baseURL, invocationID)

	resp, err := client.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Check for successful status code
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d - %s", resp.StatusCode(), resp.String())
	}

	// Decode the JSON response into a map
	var status map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &status); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return status, nil
}
