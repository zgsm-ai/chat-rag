package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/config"
)

// SemanticInterface defines the interface for semantic search client
type SemanticInterface interface {
	// Search performs semantic search and returns relevant context
	Search(ctx context.Context, req SemanticRequest) (string, error)
	// CheckReady checks if the semantic search service is available
	CheckReady(ctx context.Context, req ReadyRequest) (bool, error)
}

// SemanticRequest represents the request structure for semantic search
type SemanticRequest struct {
	ClientId      string  `json:"clientId"`
	CodebasePath  string  `json:"codebasePath"`
	Query         string  `json:"query"`
	TopK          int     `json:"topK"`
	Authorization string  `json:"authorization"`
	Score         float64 `json:"score"`
}

// ReadyRequest represents the request structure for checking service availability
type ReadyRequest struct {
	ClientId      string `json:"clientId"`
	CodebasePath  string `json:"codebasePath"`
	Authorization string `json:"authorization"`
}

// SemanticResponseWrapper represents the API standard response wrapper for semantic search
type SemanticResponseWrapper struct {
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
	searchClient *HTTPClient
	readyClient  *HTTPClient
}

// NewSemanticClient creates a new semantic client instance
func NewSemanticClient(semanticConfig config.SemanticSearchConfig) SemanticInterface {
	searchConfig := HTTPClientConfig{
		Timeout: 5 * time.Second,
	}
	readyConfig := HTTPClientConfig{
		Timeout: 3 * time.Second,
	}
	return &SemanticClient{
		searchClient: NewHTTPClient(semanticConfig.SearchEndpoint, searchConfig),
		readyClient:  NewHTTPClient(semanticConfig.ApiReadyEndpoint, readyConfig),
	}
}

// Search performs semantic search and returns relevant context
func (c *SemanticClient) Search(ctx context.Context, req SemanticRequest) (string, error) {
	// Prepare HTTP request
	httpReq := Request{
		Method:        http.MethodPost,
		Authorization: req.Authorization,
		Body:          req,
	}

	// Execute request and get raw response
	resp, err := c.searchClient.DoRequest(ctx, httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		respBody := ""
		if err == nil {
			respBody = string(body)
		}
		return "", fmt.Errorf(
			"request failed! status: %d, response:%s, url: %s",
			resp.StatusCode, respBody, resp.Request.URL.String(),
		)
	}

	// Read response body as string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// CheckReady checks if the semantic search service is available
func (c *SemanticClient) CheckReady(ctx context.Context, req ReadyRequest) (bool, error) {
	// Prepare HTTP request
	httpReq := Request{
		Method:        http.MethodGet,
		Authorization: req.Authorization,
		QueryParams: map[string]string{
			"clientId":     req.ClientId,
			"codebasePath": req.CodebasePath,
		},
	}

	// Execute request
	resp, err := c.readyClient.DoRequest(ctx, httpReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	// Read response body for error information
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %v", err)
	}

	return false, fmt.Errorf("code: %d, body: %s", resp.StatusCode, body)
}
