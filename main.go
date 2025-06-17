package main

import (
	"flag"
	"net/http"
	"strconv"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/handler"

	"github.com/gin-gonic/gin"
)

// main is the entry point of the chat-rag service
func main() {
	// Load config
	var configFile string
	flag.StringVar(&configFile, "f", "etc/chat-api.yaml", "the config file")
	flag.Parse()

	c := config.MustLoadConfig(configFile)

	// Create gin engine
	router := gin.Default()

	// Initialize service context
	ctx := bootstrap.NewServiceContext(c)

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
