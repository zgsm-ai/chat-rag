package logic

import (
	"context"
	"fmt"
	"time"

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

func (l *ChatCompletionLogic) ChatCompletion(req *types.ChatCompletionRequest) (resp *types.ChatCompletionResponse, err error) {
	startTime := time.Now()
	requestID := uuid.New().String()

	// Initialize log entry
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
		if err != nil {
			chatLog.Error = err.Error()
		}
		l.svcCtx.LoggerService.LogAsync(chatLog)
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

	var semanticStart, summaryStart time.Time
	if needsCompression {
		semanticStart = time.Now()
	}

	processedPrompt, err := processor.ProcessPrompt(l.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to process prompt: %w", err)
	}

	if needsCompression {
		chatLog.SemanticLatency = time.Since(semanticStart).Milliseconds()
		summaryStart = time.Now()
		chatLog.SummaryLatency = time.Since(summaryStart).Milliseconds()
	}

	// Update log with processed prompt info
	chatLog.IsCompressed = processedPrompt.IsCompressed
	compressedTokens := l.countTokensInMessages(processedPrompt.Messages)
	chatLog.CompressedTokens = compressedTokens

	if originalTokens > 0 {
		chatLog.CompressionRatio = float64(compressedTokens) / float64(originalTokens)
	}

	chatLog.CompressedPromptSample = model.TruncateContent(l.getPromptSample(processedPrompt.Messages), 200)

	// Generate completion using LLM
	modelStart := time.Now()

	if req.Stream {
		// Handle streaming response
		return l.handleStreamingCompletion(req, processedPrompt, chatLog, modelStart)
	} else {
		// Handle non-streaming response
		return l.handleNonStreamingCompletion(req, processedPrompt, chatLog, modelStart)
	}
}

// handleNonStreamingCompletion handles non-streaming chat completion
func (l *ChatCompletionLogic) handleNonStreamingCompletion(req *types.ChatCompletionRequest, processedPrompt *strategy.ProcessedPrompt, chatLog *model.ChatLog, modelStart time.Time) (*types.ChatCompletionResponse, error) {
	// Build prompt string from messages
	prompt := l.buildPromptFromMessages(processedPrompt.Messages)

	// Generate completion
	completion, err := l.svcCtx.SummaryModelClient.GenerateCompletion(l.ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	chatLog.MainModelLatency = time.Since(modelStart).Milliseconds()

	// Build response
	response := &types.ChatCompletionResponse{
		Id:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []types.Choice{
			{
				Index: 0,
				Message: types.Message{
					Role:    "assistant",
					Content: completion,
				},
				FinishReason: "stop",
			},
		},
		Usage: types.Usage{
			PromptTokens:     chatLog.CompressedTokens,
			CompletionTokens: l.countTokens(completion),
			TotalTokens:      chatLog.CompressedTokens + l.countTokens(completion),
		},
	}

	return response, nil
}

// handleStreamingCompletion handles streaming chat completion
func (l *ChatCompletionLogic) handleStreamingCompletion(req *types.ChatCompletionRequest, processedPrompt *strategy.ProcessedPrompt, chatLog *model.ChatLog, modelStart time.Time) (*types.ChatCompletionResponse, error) {
	// For streaming, we would need to modify the handler to support Server-Sent Events
	// For now, return a non-streaming response
	// TODO: Implement proper streaming support
	return l.handleNonStreamingCompletion(req, processedPrompt, chatLog, modelStart)
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
