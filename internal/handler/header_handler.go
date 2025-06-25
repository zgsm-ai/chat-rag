package handler

import (
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"go.uber.org/zap"
)

// getIdentityFromHeaders extracts request headers and creates Identity struct
func getIdentityFromHeaders(c *gin.Context) *model.Identity {
	clientIDE := c.GetHeader("zgsm-client-ide")
	if clientIDE == "" {
		clientIDE = "vscode"
	}

	caller := c.GetHeader("x-caller")
	if caller == "" {
		caller = "chat"
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

	jwtToken := c.GetHeader("authorization")
	userInfo := model.NewUserInfo(jwtToken)
	logger.Info("User info:", zap.Any("userInfo", userInfo))

	return &model.Identity{
		RequestID:   c.GetHeader("x-request-id"),
		TaskID:      c.GetHeader("zgsm-task-id"),
		ClientID:    c.GetHeader("zgsm-client-id"),
		ClientIDE:   clientIDE,
		ProjectPath: projectPath,
		AuthToken:   jwtToken,
		UserName:    userInfo.Name,
		LoginFrom:   userInfo.ExtractLoginFromToken(),
		Caller:      caller,
		UserInfo:    userInfo,
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
