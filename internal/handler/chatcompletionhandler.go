package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/logic"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// ChatCompletionHandler handles chat completion requests
func ChatCompletionHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.ChatCompletionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Create RequestContext
		reqCtx := &svc.RequestContext{
			Request: &req,
			Writer:  c.Writer,
			Headers: &c.Request.Header,
		}

		svcCtx.SetRequestContext(reqCtx)
		l := logic.NewChatCompletionLogic(c.Request.Context(), svcCtx)

		if req.Stream {
			// Set SSE response headers
			c.Header("Content-Type", "text/event-stream; charset=utf-8")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Transfer-Encoding", "chunked")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Headers", "Cache-Control")
			c.Header("X-Accel-Buffering", "no")

			// 立即发送响应头
			c.Status(http.StatusOK)
			flusher, ok := c.Writer.(http.Flusher)
			if ok {
				flusher.Flush()
			}

			err := l.ChatCompletionStream()
			if err != nil {
				// 对于流式响应，不能使用AbortWithStatusJSON，因为响应头已经发送
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
