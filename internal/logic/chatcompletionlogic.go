package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/strategy"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type ChatCompletionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatCompletionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatCompletionLogic {
	return &ChatCompletionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// processRequest handles common request processing logic
func (l *ChatCompletionLogic) processRequest(req *types.ChatCompletionRequest) (*model.ChatLog, *strategy.ProcessedPrompt, error) {
	startTime := time.Now()
	requestID := uuid.New().String()

	chatLog := &model.ChatLog{
		RequestID:   requestID,
		Timestamp:   startTime,
		ClientID:    req.ClientId,
		ProjectPath: req.ProjectPath,
		Model:       req.Model,
	}

	// Defer logging
	defer func() {
		chatLog.TotalLatency = time.Since(startTime).Milliseconds()
		if l.svcCtx.LoggerService != nil {
			l.svcCtx.LoggerService.LogAsync(chatLog)
		}
	}()

	// Count original tokens
	originalTokens := l.countTokensInMessages(req.Messages)
	chatLog.OriginalTokens = originalTokens
	chatLog.OriginalPromptSample = model.TruncateContent(l.getPromptSample(req.Messages), 200)

	// Determine if compression is needed
	needsCompression := l.svcCtx.Config.EnableCompression && originalTokens > l.svcCtx.Config.TokenThreshold
	chatLog.CompressionTriggered = needsCompression

	// Process prompt using strategy pattern
	processor := l.svcCtx.PromptProcessorFactory.CreateProcessor(needsCompression)

	var semanticStart time.Time
	if needsCompression {
		semanticStart = time.Now()
	}

	processedPrompt, err := processor.ProcessPrompt(l.ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process prompt: %w", err)
	}

	// Record timing information from processed prompt
	if needsCompression && processedPrompt.IsCompressed {
		chatLog.SemanticLatency = time.Since(semanticStart).Milliseconds()
		chatLog.SummaryLatency = processedPrompt.SummaryLatency
	}

	// Update log with processed prompt info
	chatLog.IsCompressed = processedPrompt.IsCompressed
	compressedTokens := l.countTokensInMessages(processedPrompt.Messages)
	chatLog.CompressedTokens = compressedTokens

	if originalTokens > 0 {
		chatLog.CompressionRatio = float64(compressedTokens) / float64(originalTokens)
	}

	chatLog.CompressedPromptSample = model.TruncateContent(l.getPromptSample(processedPrompt.Messages), 200)

	return chatLog, processedPrompt, nil
}

// ChatCompletion handles chat completion requests
func (l *ChatCompletionLogic) ChatCompletion(req *types.ChatCompletionRequest, headers http.Header) (resp *types.ChatCompletionResponse, err error) {
	chatLog, processedPrompt, err := l.processRequest(req)
	if err != nil {
		return nil, err
	}

	modelStart := time.Now()
	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.MainModelEndpoint, req.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Generate completion using structured messages
	response, err := llmClient.ChatLLMWithMessagesRaw(l.ctx, processedPrompt.Messages, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	chatLog.MainModelLatency = time.Since(modelStart).Milliseconds()
	return &response, nil
}

// ChatCompletionStream handles streaming chat completion with SSE
func (l *ChatCompletionLogic) ChatCompletionStream(req *types.ChatCompletionRequest, w http.ResponseWriter, headers http.Header) error {
	chatLog, processedPrompt, err := l.processRequest(req)
	if err != nil {
		l.sendSSEError(w, "Failed to process request", err)
		return err
	}

	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.MainModelEndpoint, req.Model)
	if err != nil {
		l.sendSSEError(w, "Failed to create LLM client", err)
		return fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Generate completion using LLM
	modelStart := time.Now()

	// Get flusher for immediate response sending
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Stream completion using structured messages with raw response
	err = llmClient.ChatLLMWithMessagesStreamRaw(l.ctx, processedPrompt.Messages, headers, func(rawLine string) error {
		// 直接发送原始行数据，不做任何处理
		if rawLine != "" {
			_, writeErr := fmt.Fprintf(w, "%s\n", rawLine)
			if writeErr != nil {
				l.Errorf("Failed to write raw stream line: %v", writeErr)
				return writeErr
			}

			// Flush immediately
			flusher.Flush()
		}

		return nil
	})

	if err != nil {
		l.sendSSEError(w, "Failed to generate streaming completion", err)
		return fmt.Errorf("failed to generate streaming completion: %w", err)
	}

	// Update chat log with completion info
	chatLog.MainModelLatency = time.Since(modelStart).Milliseconds()

	return nil
}

// Helper methods

func (l *ChatCompletionLogic) countTokensInMessages(messages []types.Message) int {
	if l.svcCtx.TokenCounter != nil {
		// Convert messages to map format for token counting
		var msgMaps []map[string]interface{}
		for _, msg := range messages {
			msgMaps = append(msgMaps, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
		return l.svcCtx.TokenCounter.CountMessagesTokens(msgMaps)
	}

	// Fallback to simple estimation
	totalText := ""
	for _, msg := range messages {
		totalText += msg.Role + ": " + msg.Content + "\n"
	}
	return utils.EstimateTokens(totalText)
}

func (l *ChatCompletionLogic) countTokens(text string) int {
	if l.svcCtx.TokenCounter != nil {
		return l.svcCtx.TokenCounter.CountTokens(text)
	}
	return utils.EstimateTokens(text)
}

func (l *ChatCompletionLogic) getPromptSample(messages []types.Message) string {
	if len(messages) == 0 {
		return ""
	}

	// Get the last user message as sample
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}

	return messages[len(messages)-1].Content
}

func (l *ChatCompletionLogic) buildPromptFromMessages(messages []types.Message) string {
	var prompt string
	for _, msg := range messages {
		prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}
	return prompt
}

// sendSSEError sends an error message in SSE format
func (l *ChatCompletionLogic) sendSSEError(w http.ResponseWriter, message string, err error) {
	l.Errorf("%s: %v", message, err)

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
		l.Errorf("Failed to marshal error response: %v", marshalErr)
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
