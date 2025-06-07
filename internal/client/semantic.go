package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// SemanticRequest represents the request structure for semantic search
type SemanticRequest struct {
	ClientId    string `json:"clientId"`
	ProjectPath string `json:"projectPath"`
	Query       string `json:"query"`
	TopK        int    `json:"topK"`
}

// SemanticResponse represents the response structure from semantic search
type SemanticResponse struct {
	Results []SemanticResult `json:"results"`
}

// SemanticResult represents a single semantic search result
type SemanticResult struct {
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	FilePath   string  `json:"filePath"`
	LineNumber int     `json:"lineNumber"`
}

// SemanticClient handles communication with the semantic search service
type SemanticClient struct {
	endpoint   string
	httpClient *http.Client
}

// NewSemanticClient creates a new semantic client instance
func NewSemanticClient(endpoint string) *SemanticClient {
	return &SemanticClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// Search performs semantic search and returns relevant context
func (c *SemanticClient) Search(ctx context.Context, req SemanticRequest) (*SemanticResponse, error) {
	log.Println("[Search] Start semantic search...")
	// Prepare request body
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("semantic search failed with status: %d", resp.StatusCode)
	}

	// Parse response
	var semanticResp SemanticResponse
	if err := json.NewDecoder(resp.Body).Decode(&semanticResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &semanticResp, nil
}
