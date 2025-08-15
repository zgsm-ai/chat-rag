package bootstrap

import (
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/functions"
	"github.com/zgsm-ai/chat-rag/internal/service"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
)

// ServiceContext holds all service dependencies
type ServiceContext struct {
	Config config.Config

	// Clients
	SemanticClient   client.SemanticInterface
	FunctionsManager *functions.ToolManager
	RedisClient      client.RedisInterface

	// Services
	LoggerService  service.LogRecordInterface
	MetricsService service.MetricsInterface

	// Utilities
	TokenCounter *tokenizer.TokenCounter

	ToolExecutor functions.ToolExecutor
}

// NewServiceContext creates a new service context with all dependencies
func NewServiceContext(c config.Config) *ServiceContext {
	// Initialize semantic client
	semanticClient := client.NewSemanticClient(c.Tools.SemanticSearch)
	relationClient := client.NewRelationClient(c.Tools.RelationSearch.SearchEndpoint)
	definitionClient := client.NewDefinitionClient(c.Tools.CodeDefinition)
	// functionManager := functions.NewToolManager("etc/functions.yaml")
	xmlToolExecutor := functions.NewXmlToolExecutor(c.Tools.SemanticSearch, semanticClient, relationClient, definitionClient)

	// Initialize token counter
	tokenCounter, err := tokenizer.NewTokenCounter()
	if err != nil {
		// Create default token counter that uses simple estimation
		panic("Failed to start NewTokenCounter:" + err.Error())
	}

	// Initialize metrics service
	metricsService := service.NewMetricsService()

	// Initialize logger service
	loggerService := service.NewLogRecordService(c)

	// Set metrics service in logger service
	loggerService.SetMetricsService(metricsService)

	// Start logger service
	if err := loggerService.Start(); err != nil {
		panic("Failed to start logger service:" + err.Error())
	}

	// Initialize Redis client
	redisClient := client.NewRedisClient(c.Redis)

	return &ServiceContext{
		Config:         c,
		SemanticClient: semanticClient,
		// FunctionsManager: functionManager,
		LoggerService:  loggerService,
		MetricsService: metricsService,
		TokenCounter:   tokenCounter,
		ToolExecutor:   xmlToolExecutor,
		RedisClient:    redisClient,
	}
}

// Stop gracefully stops all services
func (svc *ServiceContext) Stop() {
	if svc.LoggerService != nil {
		svc.LoggerService.Stop()
	}
}
