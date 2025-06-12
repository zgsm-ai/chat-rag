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

		// TODO oneapi's key, it will be removed if not use oneapi
		c.Request.Header.Set("Authorization", svcCtx.Config.OneApiAuthorization)
		// Process headers with header_handler
		identity := getIndentityFromHeaders(c)

		// Create RequestContext
		reqCtx := &svc.RequestContext{
			Request: &req,
			Writer:  c.Writer,
			Headers: &c.Request.Header,
		}

		svcCtx.SetRequestContext(reqCtx)
		l := logic.NewChatCompletionLogic(c.Request.Context(), svcCtx, identity)

		if req.Stream {
			setSSEResponseHeaders(c)

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
