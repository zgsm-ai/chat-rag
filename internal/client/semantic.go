package client

import (
	"context"
	"net/http"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// SemanticInterface defines the interface for semantic search client
type SemanticInterface interface {
	// Search performs semantic search and returns relevant context
	Search(ctx context.Context, req SemanticRequest) (string, error)
	// CheckReady checks if the semantic search service is available
	CheckReady(ctx context.Context, req ReadyRequest) (bool, error)
	// Close gracefully closes the semantic client
	Close() error
}

// SemanticRequest represents the request structure for semantic search
type SemanticRequest struct {
	ClientId      string  `json:"clientId"`
	CodebasePath  string  `json:"codebasePath"`
	Query         string  `json:"query"`
	TopK          int     `json:"topK"`
	Authorization string  `json:"authorization"`
	Score         float64 `json:"scoreThreshold"`
	ClientVersion string  `json:"clientVersion"`
}

// ReadyRequest represents the request structure for checking service availability
type ReadyRequest struct {
	ClientId      string `json:"clientId"`
	CodebasePath  string `json:"codebasePath"`
	Authorization string `json:"authorization"`
	ClientVersion string `json:"clientVersion"`
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
	*BaseClient[SemanticRequest, string]
}

// NewSemanticClient creates a new semantic client instance
func NewSemanticClient(semanticConfig config.SemanticSearchConfig) SemanticInterface {
	config := BaseClientConfig{
		SearchEndpoint: semanticConfig.SearchEndpoint,
		ReadyEndpoint:  semanticConfig.ApiReadyEndpoint,
		SearchTimeout:  5 * time.Second,
		ReadyTimeout:   5 * time.Second,
	}

	baseClient := NewBaseClient(config,
		&SemanticRequestBuilder{},
		&SemanticRequestBuilder{},
		&StringResponseHandler{},
		&StringResponseHandler{},
	)

	return &SemanticClient{
		BaseClient: baseClient,
	}
}

// Search performs semantic search and returns relevant context
func (c *SemanticClient) Search(ctx context.Context, req SemanticRequest) (string, error) {
	return c.BaseClient.Search(ctx, req)
}

// CheckReady checks if the semantic search service is available
func (c *SemanticClient) CheckReady(ctx context.Context, req ReadyRequest) (bool, error) {
	return c.BaseClient.CheckReady(ctx, req)
}

// SemanticRequestBuilder Semantic请求构建策略
type SemanticRequestBuilder struct{}

func (b *SemanticRequestBuilder) BuildRequest(req SemanticRequest) Request {
	return Request{
		Headers: map[string]string{
			types.HeaderClientVersion: req.ClientVersion,
		},
		Method:        http.MethodPost,
		Authorization: req.Authorization,
		Body:          req,
	}
}

// Close gracefully closes the semantic client
func (c *SemanticClient) Close() error {
	// BaseClient doesn't hold persistent connections that need explicit closing
	// This method is provided for interface consistency and future extensibility
	return nil
}

func (b *SemanticRequestBuilder) BuildReadyRequest(req ReadyRequest) Request {
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
