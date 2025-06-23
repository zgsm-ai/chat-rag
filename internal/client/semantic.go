package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SemanticInterface defines the interface for semantic search client
type SemanticInterface interface {
	// Search performs semantic search and returns relevant context
	Search(ctx context.Context, req SemanticRequest) (*SemanticData, error)
}

// SemanticRequest represents the request structure for semantic search
type SemanticRequest struct {
	ClientId      string `json:"clientId"`
	CodebasePath  string `json:"codebasePath"`
	Query         string `json:"query"`
	TopK          int    `json:"topK"`
	Authorization string `json:"authorization"`
}

// ResponseWrapper represents the API standard response wrapper
type ResponseWrapper struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    *SemanticData `json:"data"`
}

// SemanticData wraps the actual semantic search results
type SemanticData struct {
	Results []SemanticResult `json:"list"`
}

// SemanticResult represents a single semantic search result
type SemanticResult struct {
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
	FilePath string  `json:"filePath"`
}

// SemanticClient handles communication with the semantic search service
type SemanticClient struct {
	endpoint   string
	httpClient *http.Client
}

// NewSemanticClient creates a new semantic client instance
func NewSemanticClient(endpoint string) SemanticInterface {
	return &SemanticClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// Search performs semantic search and returns relevant context
func (c *SemanticClient) Search(ctx context.Context, req SemanticRequest) (*SemanticData, error) {
	// Create URL with query parameters
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// Add request fields as query parameters
	q := u.Query()
	q.Add("clientId", req.ClientId)
	q.Add("codebasePath", req.CodebasePath)
	q.Add("query", req.Query)
	q.Add("topK", fmt.Sprintf("%d", req.TopK))
	u.RawQuery = q.Encode()

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", req.Authorization)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		respBody := ""
		if err == nil {
			respBody = string(body)
		}
		return nil, fmt.Errorf(
			"semantic search failed! status: %d, response:%s, url: %s",
			resp.StatusCode, respBody, u.String(),
		)
	}

	// Parse response
	var wrapper ResponseWrapper
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if wrapper.Data == nil {
		return nil, fmt.Errorf("empty response data")
	}

	return wrapper.Data, nil
}
