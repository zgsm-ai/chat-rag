package main

import (
	"flag"
	"net/http"
	"strconv"

	"github.com/zgsm-ai/chat-rag/internal/utils/logger"
	"go.uber.org/zap"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/handler"
	"github.com/zgsm-ai/chat-rag/internal/svc"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// main is the entry point of the chat-rag service
func main() {
	// Load config
	var configFile string
	flag.StringVar(&configFile, "f", "etc/chat-api.yaml", "the config file")
	flag.Parse()

	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		panic("Failed to read config file: " + err.Error())
	}

	var c config.Config
	if err := viper.Unmarshal(&c); err != nil {
		panic("Failed to unmarshal config: " + err.Error())
	}

	// Create gin engine
	router := gin.Default()

	// Initialize service context
	ctx := svc.NewServiceContext(c)

	// Register routes
	handler.RegisterHandlers(router, ctx)

	// Start server
	addr := c.Host + ":" + strconv.Itoa(c.Port)
	logger.Info("server starting",
		zap.String("address", addr),
	)
	if err := http.ListenAndServe(addr, router); err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
