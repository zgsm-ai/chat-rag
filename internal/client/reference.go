package client

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// ReferenceInterface defines the interface for relation search client
type ReferenceInterface interface {
	// Search performs relation search and returns relation data
	Search(ctx context.Context, req ReferenceRequest) (string, error)
	// CheckReady checks if the relation search service is available
	CheckReady(ctx context.Context, req ReadyRequest) (bool, error)
}

// ReferenceRequest represents the request structure for relation search
type ReferenceRequest struct {
	ClientId      string `json:"clientId"`
	CodebasePath  string `json:"codebasePath"`
	FilePath      string `json:"filePath"`
	LineRange     string `json:"lineRange,omitempty"`
	SymbolName    string `json:"symbolName,omitempty"`
	MaxLayer      *int   `json:"maxLayer,omitempty"`
	Authorization string `json:"authorization"`
	ClientVersion string `json:"clientVersion"`
}

// ReferenceResponseWrapper represents the API standard response wrapper for relation search
type ReferenceResponseWrapper struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    *ReferenceData `json:"data"`
}

// ReferenceData wraps the actual relation search results
type ReferenceData struct {
	Results []ReferenceNode `json:"list"`
}

// Position represents the position in a file
type Position struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

// ReferenceNode represents a single node in the relation search result
type ReferenceNode struct {
	Content  string          `json:"content"`
	NodeType string          `json:"nodeType"`
	FilePath string          `json:"filePath"`
	Position Position        `json:"position"`
	Children []ReferenceNode `json:"children"`
}

// ReferenceClient handles communication with the relation search service
type ReferenceClient struct {
	*BaseClient[ReferenceRequest, string]
}

// NewReferenceClient creates a new relation client instance
func NewReferenceClient(referenceConfig config.ReferenceSearchConfig) ReferenceInterface {
	config := BaseClientConfig{
		SearchEndpoint: referenceConfig.SearchEndpoint,
		ReadyEndpoint:  referenceConfig.ApiReadyEndpoint,
		SearchTimeout:  5 * time.Second,
		ReadyTimeout:   5 * time.Second,
	}

	baseClient := NewBaseClient(config,
		&ReferenceRequestBuilder{},
		&ReferenceRequestBuilder{},
		&StringResponseHandler{},
		&StringResponseHandler{},
	)

	return &ReferenceClient{
		BaseClient: baseClient,
	}
}

// Search performs relation search and returns relation data
func (c *ReferenceClient) Search(ctx context.Context, req ReferenceRequest) (string, error) {
	return c.BaseClient.Search(ctx, req)
}

// CheckReady checks if the relation search service is available
func (c *ReferenceClient) CheckReady(ctx context.Context, req ReadyRequest) (bool, error) {
	return c.BaseClient.CheckReady(ctx, req)
}

// ReferenceRequestBuilder Relation请求构建策略
type ReferenceRequestBuilder struct{}

func (b *ReferenceRequestBuilder) BuildRequest(req ReferenceRequest) Request {
	queryParams := map[string]string{
		"clientId":     req.ClientId,
		"codebasePath": req.CodebasePath,
		"filePath":     req.FilePath,
	}

	// Only add lineRange if it is provided
	if req.LineRange != "" {
		queryParams["lineRange"] = req.LineRange
	}

	if req.SymbolName != "" {
		queryParams["symbolName"] = req.SymbolName
	}

	// Add maxLayer if provided
	if req.MaxLayer != nil {
		queryParams["maxLayer"] = strconv.Itoa(*req.MaxLayer)
	}

	return Request{
		Headers: map[string]string{
			types.HeaderClientVersion: req.ClientVersion,
		},
		Method:        http.MethodGet,
		QueryParams:   queryParams,
		Authorization: req.Authorization,
	}
}

func (b *ReferenceRequestBuilder) BuildReadyRequest(req ReadyRequest) Request {
	return Request{
		Headers: map[string]string{
			types.HeaderClientVersion: req.ClientVersion,
		},
		Method: http.MethodGet,
		QueryParams: map[string]string{
			"clientId":     req.ClientId,
			"codebasePath": req.CodebasePath,
		},
		Authorization: req.Authorization,
	}
}
