package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/config"
)

// DefinitionInterface defines the interface for code definition search client
type DefinitionInterface interface {
	// Search performs code definition search and returns definition details
	Search(ctx context.Context, req DefinitionRequest) (*DefinitionData, error)
	// CheckReady checks if the code definition search service is available
	CheckReady(ctx context.Context, req ReadyRequest) (bool, error)
}

// DefinitionRequest represents the request structure for code definition search
type DefinitionRequest struct {
	ClientId      string `json:"clientId"`
	CodebasePath  string `json:"codebasePath"`
	FilePath      string `json:"filePath"`
	StartLine     *int   `json:"startLine,omitempty"`
	EndLine       *int   `json:"endLine,omitempty"`
	CodeSnippet   string `json:"codeSnippet,omitempty"`
	Authorization string `json:"authorization"`
}

// DefinitionResponseWrapper represents the API standard response wrapper for code definition search
type DefinitionResponseWrapper struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    *DefinitionData `json:"data"`
}

// DefinitionData wraps the actual code definition search results
type DefinitionData struct {
	Results []DefinitionResult `json:"list"`
}

// DefinitionResult represents a single code definition search result
type DefinitionResult struct {
	FilePath string             `json:"filePath"`
	Name     string             `json:"name"`
	Type     string             `json:"type"`
	Content  string             `json:"content"`
	Position DefinitionPosition `json:"position"`
}

// DefinitionPosition represents the position information of a definition
type DefinitionPosition struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

// DefinitionClient handles communication with the code definition search service
type DefinitionClient struct {
	searchClient *HTTPClient
	readyClient  *HTTPClient
}

// NewDefinitionClient creates a new definition client instance
func NewDefinitionClient(definitionConfig config.CodeDefinitionConfig) DefinitionInterface {
	searchConfig := HTTPClientConfig{
		Timeout: 5 * time.Second,
	}
	readyConfig := HTTPClientConfig{
		Timeout: 1 * time.Second,
	}
	return &DefinitionClient{
		searchClient: NewHTTPClient(definitionConfig.SearchEndpoint, searchConfig),
		readyClient:  NewHTTPClient(definitionConfig.ApiReadyEndpoint, readyConfig),
	}
}

// Search performs code definition search and returns definition details
func (c *DefinitionClient) Search(ctx context.Context, req DefinitionRequest) (*DefinitionData, error) {
	// Prepare HTTP request
	httpReq := Request{
		Method:        http.MethodGet,
		Authorization: req.Authorization,
		QueryParams: map[string]string{
			"clientId":     req.ClientId,
			"codebasePath": req.CodebasePath,
			"filePath":     req.FilePath,
		},
	}

	// Add optional parameters if provided
	if req.StartLine != nil {
		httpReq.QueryParams["startLine"] = fmt.Sprintf("%d", *req.StartLine)
	}

	if req.EndLine != nil {
		httpReq.QueryParams["endLine"] = fmt.Sprintf("%d", *req.EndLine)
	}

	if req.CodeSnippet != "" {
		httpReq.QueryParams["codeSnippet"] = req.CodeSnippet
	}

	// Execute request using typed method
	wrapper, err := DoTypedJSONRequest[*DefinitionData](c.searchClient, ctx, httpReq)
	if err != nil {
		return nil, err
	}

	if wrapper.Data == nil {
		return nil, fmt.Errorf("empty response data")
	}

	return wrapper.Data, nil
}

// CheckReady checks if the code definition search service is available
func (c *DefinitionClient) CheckReady(ctx context.Context, req ReadyRequest) (bool, error) {
	// Prepare HTTP request
	httpReq := Request{
		Method: http.MethodGet,
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
