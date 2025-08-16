package client

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// RelationInterface defines the interface for relation search client
type RelationInterface interface {
	// Search performs relation search and returns relation data
	Search(ctx context.Context, req RelationRequest) (string, error)
	// CheckReady checks if the relation search service is available
	CheckReady(ctx context.Context, req ReadyRequest) (bool, error)
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
	*BaseClient[RelationRequest, string]
}

// NewRelationClient creates a new relation client instance
func NewRelationClient(endpoint string) RelationInterface {
	config := BaseClientConfig{
		SearchEndpoint: endpoint,
		ReadyEndpoint:  endpoint + "/ready",
		SearchTimeout:  3 * time.Second,
		ReadyTimeout:   3 * time.Second,
	}

	baseClient := NewBaseClient(config,
		&RelationRequestBuilder{},
		&RelationRequestBuilder{},
		&StringResponseHandler{},
		&StringResponseHandler{},
	)

	return &RelationClient{
		BaseClient: baseClient,
	}
}

// Search performs relation search and returns relation data
func (c *RelationClient) Search(ctx context.Context, req RelationRequest) (string, error) {
	return c.BaseClient.Search(ctx, req)
}

// CheckReady checks if the relation search service is available
func (c *RelationClient) CheckReady(ctx context.Context, req ReadyRequest) (bool, error) {
	return c.BaseClient.CheckReady(ctx, req)
}

// RelationRequestBuilder Relation请求构建策略
type RelationRequestBuilder struct{}

func (b *RelationRequestBuilder) BuildRequest(req RelationRequest) Request {
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

	return Request{
		Method:        http.MethodGet,
		QueryParams:   queryParams,
		Authorization: req.Authorization,
	}
}

func (b *RelationRequestBuilder) BuildReadyRequest(req ReadyRequest) Request {
	return Request{
		Method: http.MethodGet,
		QueryParams: map[string]string{
			"clientId":     req.ClientId,
			"codebasePath": req.CodebasePath,
		},
		Authorization: req.Authorization,
	}
}
