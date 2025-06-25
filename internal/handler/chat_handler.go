package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/logic"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// ChatCompletionHandler handles chat completion requests
func ChatCompletionHandler(svcCtx *bootstrap.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Parse and validate request
		var req types.ChatCompletionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			sendErrorResponse(c, http.StatusBadRequest, err)
			return
		}

		// 2. Process identity from headers
		identity := getIdentityFromHeaders(c)

		// 3. Prepare request context
		reqCtx := &bootstrap.RequestContext{
			Request: &req,
			Writer:  c.Writer,
			Headers: &c.Request.Header,
		}
		svcCtx.SetRequestContext(reqCtx)

		// 4. Initialize logic
		l := logic.NewChatCompletionLogic(c.Request.Context(), svcCtx, identity)

		// 5. Handle stream and non-stream cases separately
		if req.Stream {
			handleStreamResponse(c, l)
		} else {
			handleNonStreamResponse(c, l)
		}
	}
}

// handleStreamResponse handles streaming response
func handleStreamResponse(c *gin.Context, l *logic.ChatCompletionLogic) {
	setSSEResponseHeaders(c)
	c.Status(http.StatusOK)

	flusher, _ := c.Writer.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	if err := l.ChatCompletionStream(); err != nil {
		sendStreamError(c, err, flusher)
	}
}

// handleNonStreamResponse handles non-streaming response
func handleNonStreamResponse(c *gin.Context, l *logic.ChatCompletionLogic) {
	resp, err := l.ChatCompletion()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// sendErrorResponse sends a structured error response
func sendErrorResponse(c *gin.Context, statusCode int, err error) {
	c.AbortWithStatusJSON(statusCode, gin.H{
		"error": gin.H{
			"message": err.Error(),
			"type":    "api_error",
			"code":    statusCode,
		},
	})
}

// sendStreamError sends an error in streaming format
func sendStreamError(c *gin.Context, err error, flusher http.Flusher) {
	errorMsg := struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}{
		Error: struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		}{
			Message: err.Error(),
			Type:    "api_error",
		},
	}

	errorData, _ := json.Marshal(errorMsg)
	c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", errorData)))
	c.Writer.Write([]byte("data: [DONE]\n\n"))

	if flusher != nil {
		flusher.Flush()
	}
}
