package client

import (
	"context"
	"net/http"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// KnowledgeInterface defines the interface for knowledge base search client
type KnowledgeInterface interface {
	// Search performs knowledge base search and returns relevant documents
	Search(ctx context.Context, req KnowledgeRequest) (string, error)
	// CheckReady checks if the knowledge base search service is available
	CheckReady(ctx context.Context, req ReadyRequest) (bool, error)
}

// KnowledgeRequest represents the request structure for knowledge base search
type KnowledgeRequest struct {
	ClientId      string  `json:"clientId"`
	CodebasePath  string  `json:"codebasePath"`
	Query         string  `json:"query"`
	TopK          int     `json:"topK"`
	Score         float64 `json:"scoreThreshold"`
	Authorization string  `json:"authorization"`
	ClientVersion string  `json:"clientVersion"`
}

// KnowledgeResponseWrapper represents the API standard response wrapper for knowledge base search
type KnowledgeResponseWrapper struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    *KnowledgeData `json:"data"`
}

// KnowledgeData wraps the actual knowledge base search results
type KnowledgeData struct {
	Results []KnowledgeResult `json:"list"`
}

// KnowledgeResult represents a single knowledge base search result
type KnowledgeResult struct {
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
	FilePath string  `json:"filePath"`
}

// KnowledgeClient handles communication with the knowledge base search service
type KnowledgeClient struct {
	*BaseClient[KnowledgeRequest, string]
}

// NewKnowledgeClient creates a new knowledge base client instance
func NewKnowledgeClient(knowledgeConfig config.KnowledgeSearchConfig) KnowledgeInterface {
	config := BaseClientConfig{
		SearchEndpoint: knowledgeConfig.SearchEndpoint,
		ReadyEndpoint:  knowledgeConfig.ApiReadyEndpoint,
		SearchTimeout:  5 * time.Second,
		ReadyTimeout:   5 * time.Second,
	}

	baseClient := NewBaseClient(config,
		&KnowledgeRequestBuilder{},
		&KnowledgeRequestBuilder{},
		&StringResponseHandler{},
		&StringResponseHandler{},
	)

	return &KnowledgeClient{
		BaseClient: baseClient,
	}
}

// Search performs knowledge base search and returns relevant documents
func (c *KnowledgeClient) Search(ctx context.Context, req KnowledgeRequest) (string, error) {
	return c.BaseClient.Search(ctx, req)
}

// CheckReady checks if the knowledge base search service is available
func (c *KnowledgeClient) CheckReady(ctx context.Context, req ReadyRequest) (bool, error) {
	return c.BaseClient.CheckReady(ctx, req)
}

// KnowledgeRequestBuilder Knowledge request builder strategy
type KnowledgeRequestBuilder struct{}

func (b *KnowledgeRequestBuilder) BuildRequest(req KnowledgeRequest) Request {
	return Request{
		Headers: map[string]string{
			types.HeaderClientVersion: req.ClientVersion,
		},
		Method:        http.MethodPost,
		Authorization: req.Authorization,
		Body:          req,
	}
}

func (b *KnowledgeRequestBuilder) BuildReadyRequest(req ReadyRequest) Request {
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
