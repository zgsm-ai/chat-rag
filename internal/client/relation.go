package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// RelationInterface defines the interface for relation search client
type RelationInterface interface {
	// Search performs relation search and returns relation data
	Search(ctx context.Context, req RelationRequest) (string, error)
}

// RelationRequest represents the request structure for relation search
type RelationRequest struct {
	ClientId       string `json:"clientId"`
	CodebasePath   string `json:"codebasePath"`
	FilePath       string `json:"filePath"`
	StartLine      int    `json:"startLine"`
	StartColumn    int    `json:"startColumn"`
	EndLine        int    `json:"endLine"`
	EndColumn      int    `json:"endColumn"`
	SymbolName     string `json:"symbolName,omitempty"`
	IncludeContent int    `json:"includeContent,omitempty"`
	MaxLayer       int    `json:"maxLayer,omitempty"`
	Authorization  string `json:"authorization"`
}

// RelationResponseWrapper represents the API standard response wrapper for relation search
type RelationResponseWrapper struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    *RelationData `json:"data"`
}

// RelationData wraps the actual relation search results
type RelationData struct {
	Results []RelationNode `json:"list"`
}

// Position represents the position in a file
type Position struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

// RelationNode represents a single node in the relation search result
type RelationNode struct {
	Content  string         `json:"content"`
	NodeType string         `json:"nodeType"`
	FilePath string         `json:"filePath"`
	Position Position       `json:"position"`
	Children []RelationNode `json:"children"`
}

// RelationClient handles communication with the relation search service
type RelationClient struct {
	httpClient *HTTPClient
}

// NewRelationClient creates a new relation client instance
func NewRelationClient(endpoint string) RelationInterface {
	config := HTTPClientConfig{
		Timeout: 3 * time.Second,
	}
	return &RelationClient{
		httpClient: NewHTTPClient(endpoint, config),
	}
}

// Search performs relation search and returns relation data
func (c *RelationClient) Search(ctx context.Context, req RelationRequest) (string, error) {
	// Build query parameters
	queryParams := map[string]string{
		"clientId":     req.ClientId,
		"codebasePath": req.CodebasePath,
		"filePath":     req.FilePath,
		"startLine":    strconv.Itoa(req.StartLine),
		"startColumn":  strconv.Itoa(req.StartColumn),
		"endLine":      strconv.Itoa(req.EndLine),
		"endColumn":    strconv.Itoa(req.EndColumn),
	}

	if req.SymbolName != "" {
		queryParams["symbolName"] = req.SymbolName
	}

	if req.IncludeContent != 0 {
		queryParams["includeContent"] = strconv.Itoa(req.IncludeContent)
	}

	if req.MaxLayer != 0 {
		queryParams["maxLayer"] = strconv.Itoa(req.MaxLayer)
	}

	// Prepare HTTP request
	httpReq := Request{
		Method:        http.MethodGet,
		QueryParams:   queryParams,
		Authorization: req.Authorization,
	}

	// Execute request and get raw response
	resp, err := c.httpClient.DoRequest(ctx, httpReq)
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
