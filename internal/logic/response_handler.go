package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

type ResponseHandler struct {
	ctx    context.Context
	svcCtx *bootstrap.ServiceContext
}

func NewResponseHandler(ctx context.Context, svcCtx *bootstrap.ServiceContext) *ResponseHandler {
	return &ResponseHandler{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (h *ResponseHandler) extractResponseInfo(chatLog *model.ChatLog, response *types.ChatCompletionResponse) {
	logger.Info("extracting response info",
		zap.Int("choicesCount", len(response.Choices)),
	)

	// Extract response content from choices
	if len(response.Choices) > 0 {
		contentStr := utils.GetContentAsString(response.Choices[0].Message.Content)
		chatLog.ResponseContent = contentStr
	}

	// Extract usage information
	logger.Info("response usage",
		zap.Int("totalTokens", response.Usage.TotalTokens),
		zap.Int("promptTokens", response.Usage.PromptTokens),
		zap.Int("completionTokens", response.Usage.CompletionTokens),
	)

	if response.Usage.TotalTokens > 0 {
		chatLog.Usage = response.Usage
	} else {
		// Calculate usage if not provided
		chatLog.Usage = h.calculateUsage(chatLog.CompressedTokens.All, chatLog.ResponseContent)
		logger.Info("calculated usage",
			zap.Int("totalTokens", chatLog.Usage.TotalTokens),
		)
	}
}
func (h *ResponseHandler) countTokens(text string) int {
	if h.svcCtx.TokenCounter != nil {
		return h.svcCtx.TokenCounter.CountTokens(text)
	}
	return tokenizer.EstimateTokens(text)
}

// calculateUsage calculates usage information when not provided by the model
func (h *ResponseHandler) calculateUsage(promptTokens int, responseContent string) types.Usage {
	completionTokens := h.countTokens(responseContent)
	return types.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}

// extractStreamingData extracts content and usage from streaming response lines
func (h *ResponseHandler) extractStreamingData(rawLine string, responseContent *strings.Builder, finalUsage **types.Usage) {
	// Skip non-data lines
	if !strings.HasPrefix(rawLine, "data: ") {
		return
	}

	// Extract JSON data
	jsonData := strings.TrimPrefix(rawLine, "data: ")
	if jsonData == "[DONE]" {
		return
	}

	// Parse streaming chunk
	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
		logger.Error("failed to parse streaming chunk",
			zap.Error(err),
			zap.String("data", jsonData),
		)
		return
	}

	// Extract content from choices
	if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok && content != "" {
					responseContent.WriteString(content)
				}
			}
		}
	}

	// Extract usage information (usually in the last chunk)
	if usage, ok := chunk["usage"].(map[string]interface{}); ok {
		logger.Debug("found usage information in streaming response")
		*finalUsage = &types.Usage{}
		if promptTokens, ok := usage["prompt_tokens"].(float64); ok {
			(*finalUsage).PromptTokens = int(promptTokens)
		}
		if completionTokens, ok := usage["completion_tokens"].(float64); ok {
			(*finalUsage).CompletionTokens = int(completionTokens)
		}
		if totalTokens, ok := usage["total_tokens"].(float64); ok {
			(*finalUsage).TotalTokens = int(totalTokens)
		}
		logger.Debug("extracted usage",
			zap.Int("promptTokens", (*finalUsage).PromptTokens),
			zap.Int("completionTokens", (*finalUsage).CompletionTokens),
			zap.Int("totalTokens", (*finalUsage).TotalTokens),
		)
	}
}

// sendSSEError sends an error message in SSE format
func (h *ResponseHandler) sendSSEError(w http.ResponseWriter, err error) {
	logger.Warn("sending SSE error response", zap.Error(err))

	// Create error response in OpenAI format
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"message": fmt.Sprintf("%v", err),
			"type":    "server_error",
			"code":    "internal_error",
		},
	}

	errorData, marshalErr := json.Marshal(errorResponse)
	if marshalErr != nil {
		logger.Error("failed to marshal error response",
			zap.Error(marshalErr),
		)
		fmt.Fprintf(w, "data: {\"error\":{\"message\":\"Internal server error\",\"type\":\"server_error\"}}\n\n")
	} else {
		fmt.Fprintf(w, "data: %s\n\n", string(errorData))
	}

	// Send [DONE] signal to close the stream
	fmt.Fprintf(w, "data: [DONE]\n\n")

	// Flush if possible
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
