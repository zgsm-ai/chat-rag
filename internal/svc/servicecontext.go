package svc

import (
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/service"
	"github.com/zgsm-ai/chat-rag/internal/strategy"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

// ServiceContext holds all service dependencies
type ServiceContext struct {
	Config config.Config

	// Clients
	SemanticClient *client.SemanticClient
	LLMClient      *client.LLMClient

	// Services
	LoggerService *service.LoggerService

	// Utilities
	TokenCounter *utils.TokenCounter

	// Strategy factory
	PromptProcessorFactory *strategy.PromptProcessorFactory
}

// NewServiceContext creates a new service context with all dependencies
func NewServiceContext(c config.Config) *ServiceContext {
	// Initialize semantic client
	semanticClient := client.NewSemanticClient(c.SemanticApiEndpoint)

	// Initialize LLM client
	llmClient, err := client.NewLLMClient(c.MainModelEndpoint, c.SummaryModelEndpoint)
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
		c.LogBatchSize,
		c.LogScanIntervalSec,
		llmClient,
	)

	// Initialize strategy factory
	promptProcessorFactory := strategy.NewPromptProcessorFactory(
		semanticClient,
		llmClient,
		c.TopK,
	)

	// Start logger service
	loggerService.Start()

	return &ServiceContext{
		Config:                 c,
		SemanticClient:         semanticClient,
		LLMClient:              llmClient,
		LoggerService:          loggerService,
		TokenCounter:           tokenCounter,
		PromptProcessorFactory: promptProcessorFactory,
	}
}

// Stop gracefully stops all services
func (svc *ServiceContext) Stop() {
	if svc.LoggerService != nil {
		svc.LoggerService.Stop()
	}
}
