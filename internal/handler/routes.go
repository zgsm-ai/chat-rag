package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
)

func RegisterHandlers(router *gin.Engine, serverCtx *bootstrap.ServiceContext) {
	apiGroup := router.Group("/chat-rag/api")
	{
		apiGroup.POST("/v1/chat/completions", ChatCompletionHandler(serverCtx))
		apiGroup.GET("/v1/chat/requests/:requestId/status", ChatStatusHandler(serverCtx))
	}
	router.GET("/metrics", MetricsHandler(serverCtx))
}
