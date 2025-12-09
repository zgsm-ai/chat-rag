package bootstrap

import (
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/functions"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/service"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
	"go.uber.org/zap"
)

// NacosConfigResult holds the result of Nacos configuration initialization
type NacosConfigResult struct {
	NacosLoader          *config.NacosLoader
	RulesConfig          *config.RulesConfig
	ToolsConfig          *config.ToolConfig
	PreciseContextConfig *config.PreciseContextConfig
	RouterConfig         *config.RouterConfig
}

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

	// Precise Context Configuration
	PreciseContextConfig *config.PreciseContextConfig

	// Router Configuration
	RouterConfig *config.RouterConfig

	// Nacos Configuration Loader
	NacosLoader *config.NacosLoader
}

// NewServiceContext creates a new service context with all dependencies
func NewServiceContext(c config.Config) *ServiceContext {
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

	// Initialize Nacos configuration
	nacosConfig := initializeNacosConfig(c)

	// Initialize tool executor with tools configuration
	toolExecutor := functions.NewGenericToolExecutor(*nacosConfig.ToolsConfig)

	// Create service context
	svc := &ServiceContext{
		Config:               c,
		LoggerService:        loggerService,
		MetricsService:       metricsService,
		TokenCounter:         tokenCounter,
		ToolExecutor:         toolExecutor,
		RedisClient:          redisClient,
		RulesConfig:          nacosConfig.RulesConfig,
		PreciseContextConfig: nacosConfig.PreciseContextConfig,
		RouterConfig:         nacosConfig.RouterConfig,
		NacosLoader:          nacosConfig.NacosLoader,
	}

	// Start watching for configuration changes
	svc.startNacosConfigWatching()

	return svc
}

// Stop gracefully stops all services
func (svc *ServiceContext) Stop() {
	logger.Info("Starting graceful shutdown of all services...")

	// Stop logger service first to ensure all logs are processed
	if svc.LoggerService != nil {
		logger.Info("Stopping logger service...")
		svc.LoggerService.Stop()
		logger.Info("Logger service stopped")
	}

	// Close Nacos connection
	if svc.NacosLoader != nil {
		logger.Info("Closing Nacos connection...")
		if err := svc.NacosLoader.Close(); err != nil {
			logger.Error("Failed to close Nacos connection",
				zap.Error(err))
		} else {
			logger.Info("Nacos connection closed successfully")
		}
	}

	// Close Redis connections
	if svc.RedisClient != nil {
		logger.Info("Closing Redis connections...")
		if err := svc.RedisClient.Close(); err != nil {
			logger.Error("Failed to close Redis connection",
				zap.Error(err))
		} else {
			logger.Info("Redis connections closed successfully")
		}
	}

	// Note: MetricsService doesn't need explicit close as it uses Prometheus
	// which handles cleanup automatically
	// Note: TokenCounter doesn't need explicit close
	// Note: ToolExecutor cleanup is handled by individual components

	logger.Info("All services have been gracefully stopped")
}

// initializeNacosConfig initializes Nacos configuration loader and loads configurations
func initializeNacosConfig(c config.Config) *NacosConfigResult {
	// Check if Nacos is configured
	if c.Nacos.ServerAddr == "" || c.Nacos.ServerPort <= 0 {
		// If Nacos is not configured, panic immediately
		panic("Nacos is not configured properly. Please check Nacos configuration in chat-api.yaml")
	}

	logger.Info("Initializing Nacos configuration loader",
		zap.String("serverAddr", c.Nacos.ServerAddr),
		zap.Int("serverPort", c.Nacos.ServerPort))

	// Create Nacos loader
	nacosLoader, err := config.NewNacosLoader(c.Nacos)
	if err != nil {
		panic("Failed to create Nacos loader: " + err.Error())
	}

	// Load rules configuration from Nacos
	rulesConfig := &config.RulesConfig{}
	if err := nacosLoader.LoadConfig(c.Nacos.RulesDataId, rulesConfig); err != nil {
		panic("Failed to load rules configuration from Nacos: " + err.Error())
	}

	// Load tools configuration from Nacos
	toolsConfig := &config.ToolConfig{}
	if err := nacosLoader.LoadConfig(c.Nacos.ToolsDataId, toolsConfig); err != nil {
		panic("Failed to load tools configuration from Nacos: " + err.Error())
	}

	// Load precise context configuration from Nacos
	preciseContextConfig := &config.PreciseContextConfig{}
	if err := nacosLoader.LoadConfig(c.Nacos.PreciseContextDataId, preciseContextConfig); err != nil {
		panic("Failed to load precise context configuration from Nacos: " + err.Error())
	}

	// Load router configuration from Nacos
	routerConfig := &config.RouterConfig{}
	if err := nacosLoader.LoadConfig(c.Nacos.RouterDataId, routerConfig); err != nil {
		panic("Failed to load router configuration from Nacos: " + err.Error())
	}

	logger.Info("Nacos configuration initialization completed successfully",
		zap.String("serverAddr", c.Nacos.ServerAddr),
		zap.Int("serverPort", c.Nacos.ServerPort),
		zap.Int("agentsCount", len(rulesConfig.Agents)),
		zap.Int("toolsCount", len(toolsConfig.GenericTools)),
		zap.Int("agentsMatchCount", len(preciseContextConfig.AgentsMatch)),
		zap.Bool("routerEnabled", routerConfig.Enabled))

	return &NacosConfigResult{
		NacosLoader:          nacosLoader,
		RulesConfig:          rulesConfig,
		ToolsConfig:          toolsConfig,
		PreciseContextConfig: preciseContextConfig,
		RouterConfig:         routerConfig,
	}
}

// startNacosConfigWatching starts watching for Nacos configuration changes
func (svc *ServiceContext) startNacosConfigWatching() {
	if svc.NacosLoader == nil {
		logger.Warn("Nacos loader is not initialized, skipping configuration watching")
		return
	}
	// 注册 RulesConfig Handler
	err := svc.NacosLoader.RegisterGenericConfig(
		svc.Config.Nacos.RulesDataId,
		&config.RulesConfig{},
		func(data interface{}) {
			if rulesConfig, ok := data.(*config.RulesConfig); ok {
				svc.updateRulesConfig(rulesConfig)
			}
		},
	)
	if err != nil {
		panic("Failed to register rules config handler: " + err.Error())
	}

	// 注册 ToolsConfig Handler
	err = svc.NacosLoader.RegisterGenericConfig(
		svc.Config.Nacos.ToolsDataId,
		&config.ToolConfig{},
		func(data interface{}) {
			if toolsConfig, ok := data.(*config.ToolConfig); ok {
				logger.Info("Recreating tool executor with new tools configuration")
				newToolExecutor := functions.NewGenericToolExecutor(*toolsConfig)
				svc.updateToolExecutor(newToolExecutor)
				logger.Info("Tool executor successfully recreated with new configuration")
			}
		},
	)
	if err != nil {
		panic("Failed to register tools config handler: " + err.Error())
	}

	// 注册 PreciseContextConfig Handler
	err = svc.NacosLoader.RegisterGenericConfig(
		svc.Config.Nacos.PreciseContextDataId,
		&config.PreciseContextConfig{},
		func(data interface{}) {
			if preciseContextConfig, ok := data.(*config.PreciseContextConfig); ok {
				svc.updatePreciseContextConfig(preciseContextConfig)
			}
		},
	)
	if err != nil {
		panic("Failed to register precise context config handler: " + err.Error())
	}

	// 注册 RouterConfig Handler
	err = svc.NacosLoader.RegisterGenericConfig(
		svc.Config.Nacos.RouterDataId,
		&config.RouterConfig{},
		func(data interface{}) {
			if routerConfig, ok := data.(*config.RouterConfig); ok {
				svc.updateRouterConfig(routerConfig)
			}
		},
	)
	if err != nil {
		panic("Failed to register router config handler: " + err.Error())
	}

	// Start watching for configuration changes (using new parameterless method)
	err = svc.NacosLoader.StartWatching()
	if err != nil {
		panic("Failed to start watching for configuration changes: " + err.Error())
	} else {
		logger.Info("Successfully started watching for Nacos configuration changes")
	}
}

// updateRulesConfig updates the rules configuration
func (svc *ServiceContext) updateRulesConfig(rulesConfig *config.RulesConfig) {
	svc.RulesConfig = rulesConfig
	logger.Info("Rules configuration updated successfully",
		zap.Int("agentsCount", len(rulesConfig.Agents)))
}

// updateToolExecutor updates the tool executor
func (svc *ServiceContext) updateToolExecutor(toolExecutor functions.ToolExecutor) {
	svc.ToolExecutor = toolExecutor
	logger.Info("Tool executor updated successfully")
}

// updatePreciseContextConfig updates the precise context configuration
func (svc *ServiceContext) updatePreciseContextConfig(preciseContextConfig *config.PreciseContextConfig) {
	svc.PreciseContextConfig = preciseContextConfig
	logger.Info("Precise context configuration updated successfully",
		zap.Int("agentsMatchCount", len(preciseContextConfig.AgentsMatch)),
		zap.Bool("envDetailsFilterEnabled", preciseContextConfig.EnableEnvDetailsFilter))
}

// updateRouterConfig updates the router configuration
func (svc *ServiceContext) updateRouterConfig(routerConfig *config.RouterConfig) {
	svc.RouterConfig = routerConfig
	logger.Info("Router configuration updated successfully",
		zap.Bool("enabled", routerConfig.Enabled),
		zap.String("strategy", routerConfig.Strategy))
}
