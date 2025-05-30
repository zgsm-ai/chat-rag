package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/svc"
)

func RegisterHandlers(router *gin.Engine, serverCtx *svc.ServiceContext) {
	router.POST("/v1/chat/completions", ChatCompletionHandler(serverCtx))
}
