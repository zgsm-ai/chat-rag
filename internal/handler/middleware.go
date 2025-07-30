package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/zgsm-ai/chat-rag/internal/model"
)

// IdentityMiddleware is an optional authentication middleware
// It extracts identity information from request headers and stores it in context
func IdentityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract identity information from request headers
		identity := getIdentityFromHeaders(c)

		// Store identity information in context
		ctxWithIdentity := context.WithValue(c.Request.Context(), model.IdentityContextKey, identity)
		c.Request = c.Request.WithContext(ctxWithIdentity)

		// Continue processing the request
		c.Next()
	}
}
