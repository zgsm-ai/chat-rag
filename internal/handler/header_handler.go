package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

// getIndentityFromHeaders extracts request headers and creates Identity struct
func getIndentityFromHeaders(c *gin.Context) *types.Identity {
	return &types.Identity{
		RequestID:   c.GetHeader("x-request-id"),
		TaskID:      c.GetHeader("zgsm-task-id"),
		ClientID:    c.GetHeader("zgsm-client-id"),
		ProjectPath: c.GetHeader("zgsm-project-path"),
		AuthToken:   c.GetHeader("authorization"),
		UserName:    utils.ExtractUserNameFromToken(c.GetHeader("authorization")),
	}
}

// setSSEResponseHeaders sets SSE response headers
func setSSEResponseHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")
	c.Header("X-Accel-Buffering", "no")
}
