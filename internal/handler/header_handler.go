package handler

import (
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"github.com/zgsm-ai/chat-rag/internal/utils/logger"
	"go.uber.org/zap"
)

// getIndentityFromHeaders extracts request headers and creates Identity struct
func getIndentityFromHeaders(c *gin.Context) *types.Identity {
	clientIDE := c.GetHeader("zgsm-client-ide")
	if clientIDE == "" {
		clientIDE = "vscode"
	}

	projectPath := c.GetHeader("zgsm-project-path")
	decodedPath, err := url.PathUnescape(projectPath)
	if err != nil {
		logger.Error("Failed to PathUnescape project path",
			zap.String("projectPath", projectPath),
			zap.Error(err),
		)
	} else {
		projectPath = decodedPath
	}

	return &types.Identity{
		RequestID:   c.GetHeader("x-request-id"),
		TaskID:      c.GetHeader("zgsm-task-id"),
		ClientID:    c.GetHeader("zgsm-client-id"),
		ClientIDE:   clientIDE,
		ProjectPath: projectPath,
		AuthToken:   c.GetHeader("authorization"),
		UserName:    utils.ExtractUserNameFromToken(c.GetHeader("authorization")),
		LoginFrom:   utils.ExtractLoginFromToken(c.GetHeader("authorization")),
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
