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
	RedisClient client.RedisInterface

	// Services
	LoggerService  service.LogRecordInterface
	MetricsService service.MetricsInterface

	// Utilities
	TokenCounter *tokenizer.TokenCounter

	ToolExecutor functions.ToolExecutor

	// Rules Configuration
	RulesConfig *config.RulesConfig
}

// NewServiceContext creates a new service context with all dependencies
func NewServiceContext(c config.Config) *ServiceContext {
	// Initialize tool executor with universal tools
	toolExecutor := functions.NewGenericToolExecutor(c.Tools)

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

	// Load rules configuration
	rulesConfig, err := config.LoadRulesConfig()
	if err != nil {
		panic("Failed to load rules configuration:" + err.Error())
	}

	return &ServiceContext{
		Config:         c,
		LoggerService:  loggerService,
		MetricsService: metricsService,
		TokenCounter:   tokenCounter,
		ToolExecutor:   toolExecutor,
		RedisClient:    redisClient,
		RulesConfig:    rulesConfig,
	}
}

// Stop gracefully stops all services
func (svc *ServiceContext) Stop() {
	if svc.LoggerService != nil {
		svc.LoggerService.Stop()
	}
}
