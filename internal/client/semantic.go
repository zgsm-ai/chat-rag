package client

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// SemanticInterface defines the interface for semantic search client
type SemanticInterface interface {
	// Search performs semantic search and returns relevant context
	Search(ctx context.Context, req SemanticRequest) (*SemanticData, error)
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
	httpClient *HTTPClient
}

// NewSemanticClient creates a new semantic client instance
func NewSemanticClient(endpoint string) SemanticInterface {
	config := HTTPClientConfig{
		Timeout: 3 * time.Second,
	}
	return &SemanticClient{
		httpClient: NewHTTPClient(endpoint, config),
	}
}

// Search performs semantic search and returns relevant context
func (c *SemanticClient) Search(ctx context.Context, req SemanticRequest) (*SemanticData, error) {
	// Prepare HTTP request
	httpReq := Request{
		Method:        http.MethodPost,
		Authorization: req.Authorization,
		Body:          req,
	}

	// Execute request using typed method
	wrapper, err := DoTypedJSONRequest[*SemanticData](c.httpClient, ctx, httpReq)
	if err != nil {
		return nil, err
	}

	if wrapper.Data == nil {
		return nil, fmt.Errorf("empty response data")
	}

	return wrapper.Data, nil
}
