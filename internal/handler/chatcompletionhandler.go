package handler

import (
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

		// 创建 RequestContext
		reqCtx := &svc.RequestContext{
			Request: &req,
			Writer:  c.Writer,
			Headers: &c.Request.Header,
		}

		svcCtx.SetRequestContext(reqCtx)
		l := logic.NewChatCompletionLogic(c.Request.Context(), svcCtx)

		if req.Stream {
			flusher, ok := c.Writer.(http.Flusher)
			if ok {
				flusher.Flush()
			}

			err := l.ChatCompletionStream()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
