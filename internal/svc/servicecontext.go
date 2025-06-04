package svc

import (
	"net/http"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/service"
	"github.com/zgsm-ai/chat-rag/internal/strategy"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

type RequestContext struct {
	Request *types.ChatCompletionRequest
	Writer  http.ResponseWriter
	Headers *http.Header
}

// ServiceContext holds all service dependencies
type ServiceContext struct {
	Config config.Config

	// Clients
	SemanticClient *client.SemanticClient

	// Services
	LoggerService *service.LoggerService

	// Utilities
	TokenCounter *utils.TokenCounter

	// Strategy factory
	PromptProcessorFactory *strategy.PromptProcessorFactory

	// Request context
	ReqCtx *RequestContext
}

// NewServiceContext creates a new service context with all dependencies
func NewServiceContext(c config.Config) *ServiceContext {
	// Initialize semantic client
	semanticClient := client.NewSemanticClient(c.SemanticApiEndpoint)

	// Initialize LLM client
	summaryModelClient, err := client.NewLLMClient(c.LLMEndpoint, c.SummaryModel)
	if err != nil {
		panic("Failed to initialize LLM client: " + err.Error())
	}

	// Initialize token counter
	tokenCounter, err := utils.NewTokenCounter()
	if err != nil {
		// Fallback to simple estimation if tiktoken fails
		tokenCounter = nil
	}

	// Initialize logger service
	loggerService := service.NewLoggerService(
		c.LogFilePath,
		c.LokiEndpoint,
		c.LogScanIntervalSec,
		summaryModelClient, // TODO: change to classify model client
	)

	// Initialize strategy factory
	promptProcessorFactory := strategy.NewPromptProcessorFactory(
		semanticClient,
		summaryModelClient,
		c.TopK,
	)

	// Start logger service
	if err := loggerService.Start(); err != nil {
		panic("Failed to start logger service:" + err.Error())
	}

	return &ServiceContext{
		Config:                 c,
		SemanticClient:         semanticClient,
		LoggerService:          loggerService,
		TokenCounter:           tokenCounter,
		PromptProcessorFactory: promptProcessorFactory,
	}
}

func (svc *ServiceContext) SetRequestContext(reqCtx *RequestContext) {
	svc.ReqCtx = reqCtx
}

// Stop gracefully stops all services
func (svc *ServiceContext) Stop() {
	if svc.LoggerService != nil {
		svc.LoggerService.Stop()
	}
}
