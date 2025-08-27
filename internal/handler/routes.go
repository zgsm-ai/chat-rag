package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
)

func RegisterHandlers(router *gin.Engine, serverCtx *bootstrap.ServiceContext) {
	apiGroup := router.Group("/chat-rag/api")
	{
		// 为需要身份验证的路由应用中间件
		apiGroup.POST("/v1/chat/completions", IdentityMiddleware(), ChatCompletionHandler(serverCtx))
		apiGroup.GET("/v1/chat/requests/:requestId/status", ChatStatusHandler(serverCtx))
	}
	router.GET("/metrics", MetricsHandler(serverCtx))
}
