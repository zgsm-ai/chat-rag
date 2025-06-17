package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/strategy"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

type ChatCompletionLogic struct {
	ctx      context.Context
	svcCtx   *bootstrap.ServiceContext
	identity *types.Identity
}

func NewChatCompletionLogic(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	identity *types.Identity,
) *ChatCompletionLogic {
	return &ChatCompletionLogic{
		ctx:      ctx,
		svcCtx:   svcCtx,
		identity: identity,
	}
}

func (l *ChatCompletionLogic) getHeaders() *http.Header {
	return l.svcCtx.ReqCtx.Headers
}

func (l *ChatCompletionLogic) getRequest() *types.ChatCompletionRequest {
	return l.svcCtx.ReqCtx.Request
}

func (l *ChatCompletionLogic) getWriter() http.ResponseWriter {
	return l.svcCtx.ReqCtx.Writer
}

// processRequest handles common request processing logic
func (l *ChatCompletionLogic) processRequest(req *types.ChatCompletionRequest) (*model.ChatLog, *strategy.ProcessedPrompt, error) {
	logger.Info("start to process request",
		zap.String("user", l.identity.UserName),
	)
	startTime := time.Now()

	// Initialize chat log
	chatLog := l.initializeChatLog(startTime, req)

	promptProcessor := strategy.NewPromptProcessor(l.ctx, l.svcCtx, l.getRequest().ExtraBody.PromptMode, l.identity)
	processedPrompt, err := promptProcessor.Process(req.Messages)
	if err != nil {
		err := fmt.Errorf("failed to process prompt:\n %w", err)
		chatLog.AddError(types.ErrExtra, err)
		logger.Error("failed to process prompt",
			zap.Error(err),
		)
		return chatLog, nil, err
	}

	// Update chat log with processed prompt info
	l.updateChatLogWithProcessedPrompt(chatLog, processedPrompt)

	return chatLog, processedPrompt, nil
}

// initializeChatLog creates and initializes a new ChatLog with basic information and original token stats
func (l *ChatCompletionLogic) initializeChatLog(startTime time.Time, req *types.ChatCompletionRequest) *model.ChatLog {
	// Count original tokens
	userMessageTokens := l.countTokensInMessages(utils.GetUserMsgs(req.Messages))
	originalAllMessageTokens := l.countTokensInMessages(req.Messages)
	systemMessageTokens := originalAllMessageTokens - userMessageTokens

	return &model.ChatLog{
		Identity:  *l.identity,
		Timestamp: startTime,
		Model:     req.Model,
		OriginalTokens: model.TokenStats{
			SystemTokens: systemMessageTokens,
			UserTokens:   userMessageTokens,
			All:          originalAllMessageTokens,
		},
		OriginalPrompt: req.Messages,
	}
}

// updateChatLogWithProcessedPrompt updates the chat log with information from the processed prompt
func (l *ChatCompletionLogic) updateChatLogWithProcessedPrompt(chatLog *model.ChatLog, processedPrompt *strategy.ProcessedPrompt) {
	// Record timing information from processed prompt
	chatLog.SemanticLatency = processedPrompt.SemanticLatency
	chatLog.SummaryLatency = processedPrompt.SummaryLatency
	chatLog.SemanticContext = processedPrompt.SemanticContext

	// Update log with processed prompt info
	chatLog.IsUserPromptCompressed = processedPrompt.IsCompressed
	compressedAllTokens := l.countTokensInMessages(processedPrompt.Messages)
	compressedUserTokens := l.countTokensInMessages(utils.GetUserMsgs(processedPrompt.Messages))
	compressedSystemTokens := compressedAllTokens - compressedUserTokens

	chatLog.CompressedTokens = model.TokenStats{
		SystemTokens: compressedSystemTokens,
		UserTokens:   compressedUserTokens,
		All:          compressedAllTokens,
	}

	// Calculate compression ratio
	if chatLog.OriginalTokens.All > 0 {
		ratio := float64(compressedAllTokens) / float64(chatLog.OriginalTokens.All)
		chatLog.CompressionRatio, _ = strconv.ParseFloat(strconv.FormatFloat(ratio, 'f', 2, 64), 64)
	}

	chatLog.CompressedPrompt = processedPrompt.Messages

	if processedPrompt.SemanticErr != nil {
		chatLog.AddError(types.ErrSemantic, processedPrompt.SemanticErr)
	}

	if processedPrompt.SummaryErr != nil {
		chatLog.AddError(types.ErrSummary, processedPrompt.SummaryErr)
	}
}

// ChatCompletion handles chat completion requests
func (l *ChatCompletionLogic) ChatCompletion() (resp *types.ChatCompletionResponse, err error) {
	chatLog, processedPrompt, err := l.processRequest(l.getRequest())
	processedMsgs := l.getRequest().Messages
	if err != nil {
		err := fmt.Errorf("ChatCompletion failed to process request:\n%w", err)
		logger.Error("failed to process request",
			zap.Error(err),
		)
		chatLog.AddError(types.ErrExtra, err)
		chatLog.IsPromptProceed = false
	} else {
		chatLog.IsPromptProceed = true
		processedMsgs = processedPrompt.Messages
	}

	// Defer logging for non-streaming requests
	defer func() {
		chatLog.TotalLatency = time.Since(chatLog.Timestamp).Milliseconds()
		if l.svcCtx.LoggerService != nil {
			l.svcCtx.LoggerService.LogAsync(chatLog, l.getHeaders())
		}
	}()

	modelStart := time.Now()
	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLMEndpoint, l.getRequest().Model, l.getHeaders())
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Generate completion using structured messages
	response, err := llmClient.ChatLLMWithMessagesRaw(l.ctx, processedMsgs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	chatLog.MainModelLatency = time.Since(modelStart).Milliseconds()

	// Extract response content and usage information
	l.extractResponseInfo(chatLog, &response)

	return &response, nil
}

// ChatCompletionStream handles streaming chat completion with SSE
func (l *ChatCompletionLogic) ChatCompletionStream() error {
	chatLog, processedPrompt, err := l.processRequest(l.getRequest())
	processedMsgs := l.getRequest().Messages
	if err != nil {
		err := fmt.Errorf("ChatCompletionStream failed to process request: %w", err)
		logger.Error("failed to process request in streaming",
			zap.Error(err),
		)
		chatLog.AddError(types.ErrExtra, err)
		chatLog.IsPromptProceed = false
	} else {
		chatLog.IsPromptProceed = true
		processedMsgs = processedPrompt.Messages
	}

	// Defer logging for streaming requests - will be called after streaming completes
	defer func() {
		chatLog.TotalLatency = time.Since(chatLog.Timestamp).Milliseconds()
		if l.svcCtx.LoggerService != nil {
			l.svcCtx.LoggerService.LogAsync(chatLog, l.getHeaders())
		}
	}()

	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLMEndpoint, l.getRequest().Model, l.getHeaders())
	if err != nil {
		l.sendSSEError(l.getWriter(), "Failed to create LLM client", err)
		return fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Generate completion using LLM
	modelStart := time.Now()

	// Get flusher for immediate response sending
	flusher, ok := l.getWriter().(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Variables to collect streaming response data
	var responseContent strings.Builder
	var finalUsage *types.Usage

	// Stream completion using structured messages with raw response
	err = llmClient.ChatLLMWithMessagesStreamRaw(l.ctx, processedMsgs, func(rawLine string) error {
		// Handle raw line data, ensure correct SSE format
		if rawLine != "" {
			// Extract content and usage from streaming data
			l.extractStreamingData(rawLine, &responseContent, &finalUsage)

			// Ensure correct SSE format: if rawLine already has 'data: ' prefix, use directly; otherwise add prefix
			var sseData string
			if strings.HasPrefix(rawLine, "data: ") {
				sseData = rawLine
			} else {
				sseData = "data: " + rawLine
			}

			// Output SSE formatted data, ensuring correct line breaks
			_, writeErr := fmt.Fprintf(l.getWriter(), "%s\n\n", sseData)
			if writeErr != nil {
				logger.Error("failed to write SSE stream line",
					zap.Error(writeErr),
				)
				return writeErr
			}

			// Flush buffer immediately
			flusher.Flush()
		}

		return nil
	})

	if err != nil {
		l.sendSSEError(l.getWriter(), "Failed to generate streaming completion", err)
		return fmt.Errorf("failed to generate streaming completion: %w", err)
	}

	// Update chat log with completion info
	chatLog.MainModelLatency = time.Since(modelStart).Milliseconds()

	// Set response content and usage information
	responseText := responseContent.String()
	chatLog.ResponseContent = responseText

	if finalUsage != nil {
		chatLog.Usage = *finalUsage
	} else {
		// Calculate usage if not provided in streaming response
		chatLog.Usage = l.calculateUsage(chatLog.CompressedTokens.All, responseText)
		logger.Info("calculated usage for streaming response",
			zap.Int("totalTokens", chatLog.Usage.TotalTokens),
		)
	}

	// Send stream response end marker
	// fmt.Fprintf(l.getWriter(), "data: [DONE]\n\n")
	flusher.Flush()

	return nil
}

// Helper methods

func (l *ChatCompletionLogic) countTokensInMessages(messages []types.Message) int {
	if l.svcCtx.TokenCounter != nil {
		return l.svcCtx.TokenCounter.CountMessagesTokens(messages)
	}

	// Fallback to simple estimation
	totalText := ""
	for _, msg := range messages {
		totalText += msg.Role + ": " + utils.GetContentAsString(msg.Content) + "\n"
	}
	return tokenizer.EstimateTokens(totalText)
}

func (l *ChatCompletionLogic) countTokens(text string) int {
	if l.svcCtx.TokenCounter != nil {
		return l.svcCtx.TokenCounter.CountTokens(text)
	}
	return tokenizer.EstimateTokens(text)
}

// sendSSEError sends an error message in SSE format
func (l *ChatCompletionLogic) sendSSEError(w http.ResponseWriter, message string, err error) {
	logger.Error(message,
		zap.Error(err),
	)

	// Create error response in OpenAI format
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"message": fmt.Sprintf("%s: %v", message, err),
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

// extractResponseInfo extracts response content and usage from non-streaming response
func (l *ChatCompletionLogic) extractResponseInfo(chatLog *model.ChatLog, response *types.ChatCompletionResponse) {
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
		chatLog.Usage = l.calculateUsage(chatLog.CompressedTokens.All, chatLog.ResponseContent)
		logger.Info("calculated usage",
			zap.Int("totalTokens", chatLog.Usage.TotalTokens),
		)
	}
}

// extractStreamingData extracts content and usage from streaming response lines
func (l *ChatCompletionLogic) extractStreamingData(rawLine string, responseContent *strings.Builder, finalUsage **types.Usage) {
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

// calculateUsage calculates usage information when not provided by the model
func (l *ChatCompletionLogic) calculateUsage(promptTokens int, responseContent string) types.Usage {
	completionTokens := l.countTokens(responseContent)
	return types.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}
