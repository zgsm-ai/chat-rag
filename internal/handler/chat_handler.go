package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/logic"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

// ChatCompletionHandler handles chat completion requests
func ChatCompletionHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.ChatCompletionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get headers info
		identity := &types.Identity{
			RequestID:   c.GetHeader("x-request-id"),
			TaskID:      c.GetHeader("zgsm-task-id"),
			ClientID:    c.GetHeader("zgsm-client-id"),
			ProjectPath: c.GetHeader("zgsm-project-path"),
			UserName:    utils.ExtractUserNameFromToken(c.GetHeader("x-access-token")),
		}

		// Create RequestContext
		reqCtx := &svc.RequestContext{
			Request: &req,
			Writer:  c.Writer,
			Headers: &c.Request.Header,
		}

		svcCtx.SetRequestContext(reqCtx)
		l := logic.NewChatCompletionLogic(c.Request.Context(), svcCtx, identity)

		// TODO Set Authorization header for oneapi, it will be deleted if oneapi not used
		c.Header("Authorization", svcCtx.Config.OneApiAuthorization)

		if req.Stream {
			// Set SSE response headers
			c.Header("Content-Type", "text/event-stream; charset=utf-8")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Transfer-Encoding", "chunked")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Headers", "Cache-Control")
			c.Header("X-Accel-Buffering", "no")

			// Send response headers immediately
			c.Status(http.StatusOK)
			flusher, ok := c.Writer.(http.Flusher)
			if ok {
				flusher.Flush()
			}

			err := l.ChatCompletionStream()
			if err != nil {
				// For streaming responses, cannot use AbortWithStatusJSON because headers are already sent
				c.Writer.Write([]byte(fmt.Sprintf("data: {\"error\":{\"message\":\"%s\"}}\n\n", err.Error())))
				c.Writer.Write([]byte("data: [DONE]\n\n"))
				if flusher != nil {
					flusher.Flush()
				}
			}
		} else {
			resp, err := l.ChatCompletion()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(http.StatusOK, resp)
			}
		}
	}
}
