package svc

import (
	"log"
	"net/http"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/service"
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
	SemanticClient client.SemanticInterface

	// Services
	LoggerService  *service.LoggerService
	MetricsService *service.MetricsService

	// Utilities
	TokenCounter *utils.TokenCounter

	// Request context
	ReqCtx *RequestContext
}

// NewServiceContext creates a new service context with all dependencies
func NewServiceContext(c config.Config) *ServiceContext {
	// Initialize semantic client
	semanticClient := client.NewSemanticClient(c.SemanticApiEndpoint)

	// Initialize token counter
	tokenCounter, err := utils.NewTokenCounter()
	if err != nil {
		// Create default token counter that uses simple estimation
		log.Printf("Failed to initialize token encoder, using fallback estimation: %v", err)
		panic("Failed to start NewTokenCounter:" + err.Error())
	}

	// Initialize metrics service
	metricsService := service.NewMetricsService()

	// Initialize logger service
	loggerService := service.NewLoggerService(c)

	// Set metrics service in logger service
	loggerService.SetMetricsService(metricsService)

	// Start logger service
	if err := loggerService.Start(); err != nil {
		panic("Failed to start logger service:" + err.Error())
	}

	return &ServiceContext{
		Config:         c,
		SemanticClient: semanticClient,
		LoggerService:  loggerService,
		MetricsService: metricsService,
		TokenCounter:   tokenCounter,
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
