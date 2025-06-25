package logic

import (
	"context"
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
	ctx             context.Context
	svcCtx          *bootstrap.ServiceContext
	identity        *model.Identity
	responseHandler *ResponseHandler
}

func NewChatCompletionLogic(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	identity *model.Identity,
) *ChatCompletionLogic {
	return &ChatCompletionLogic{
		ctx:             ctx,
		svcCtx:          svcCtx,
		identity:        identity,
		responseHandler: NewResponseHandler(ctx, svcCtx),
	}
}

func (l *ChatCompletionLogic) headers() *http.Header {
	return l.svcCtx.ReqCtx.Headers
}

func (l *ChatCompletionLogic) request() *types.ChatCompletionRequest {
	return l.svcCtx.ReqCtx.Request
}

func (l *ChatCompletionLogic) writer() http.ResponseWriter {
	return l.svcCtx.ReqCtx.Writer
}

// processRequest handles common request processing logic
func (l *ChatCompletionLogic) processRequest() (*model.ChatLog, *strategy.ProcessedPrompt, error) {
	logger.Info("start to process request", zap.String("user", l.identity.UserName))
	startTime := time.Now()

	// Initialize chat log
	chatLog := l.newChatLog(startTime)

	promptProcessor := strategy.NewPromptProcessor(l.ctx, l.svcCtx, l.request().ExtraBody.PromptMode, l.identity)
	processedPrompt, err := promptProcessor.Process(l.request().Messages)
	if err != nil {
		err := fmt.Errorf("failed to process prompt:\n %w", err)
		chatLog.AddError(types.ErrExtra, err)
		logger.Error("failed to process prompt", zap.Error(err))
		return chatLog, nil, err
	}

	// Update chat log with processed prompt info
	l.updateChatLog(chatLog, processedPrompt)
	return chatLog, processedPrompt, nil
}

func (l *ChatCompletionLogic) newChatLog(startTime time.Time) *model.ChatLog {
	userTokens := l.countTokensInMessages(utils.GetUserMsgs(l.request().Messages))
	allTokens := l.countTokensInMessages(l.request().Messages)

	return &model.ChatLog{
		Identity:  *l.identity,
		Timestamp: startTime,
		Model:     l.request().Model,
		OriginalTokens: model.TokenStats{
			SystemTokens: allTokens - userTokens,
			UserTokens:   userTokens,
			All:          allTokens,
		},
		OriginalPrompt: l.request().Messages,
	}
}

// updateChatLog updates the chat log with information from the processed prompt
func (l *ChatCompletionLogic) updateChatLog(chatLog *model.ChatLog, processedPrompt *strategy.ProcessedPrompt) {
	// Record timing information from processed prompt
	chatLog.SemanticLatency = processedPrompt.SemanticLatency
	chatLog.SummaryLatency = processedPrompt.SummaryLatency
	chatLog.SemanticContext = processedPrompt.SemanticContext

	// Update log with processed prompt info
	chatLog.IsUserPromptCompressed = processedPrompt.IsCompressed
	allTokens := l.countTokensInMessages(processedPrompt.Messages)
	userTokens := l.countTokensInMessages(utils.GetUserMsgs(processedPrompt.Messages))

	chatLog.CompressedTokens = model.TokenStats{
		SystemTokens: allTokens - userTokens,
		UserTokens:   userTokens,
		All:          allTokens,
	}

	// Calculate compression ratio
	if chatLog.OriginalTokens.All > 0 {
		ratio := float64(allTokens) / float64(chatLog.OriginalTokens.All)
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

func (l *ChatCompletionLogic) logCompletion(chatLog *model.ChatLog) {
	chatLog.TotalLatency = time.Since(chatLog.Timestamp).Milliseconds()
	if l.svcCtx.LoggerService != nil {
		l.svcCtx.LoggerService.LogAsync(chatLog, l.svcCtx.ReqCtx.Headers)
	}
}

// ChatCompletion handles chat completion requests
func (l *ChatCompletionLogic) ChatCompletion() (resp *types.ChatCompletionResponse, err error) {
	chatLog, processedPrompt, err := l.processRequest()
	msgs := l.request().Messages

	defer l.logCompletion(chatLog)

	if err == nil {
		msgs = processedPrompt.Messages
		chatLog.IsPromptProceed = true
	} else {
		err := fmt.Errorf("ChatCompletion failed to process request:\n%w", err)
		logger.Error("failed to process request", zap.Error(err))
		chatLog.AddError(types.ErrExtra, err)
		chatLog.IsPromptProceed = false
	}

	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLMEndpoint, l.request().Model, l.headers())
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	modelStart := time.Now()
	// Generate completion using structured messages
	response, err := llmClient.ChatLLMWithMessagesRaw(l.ctx, msgs)
	if err != nil {
		chatLog.AddError(types.ErrExtra, err)
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	chatLog.MainModelLatency = time.Since(modelStart).Milliseconds()

	// Extract response content and usage information
	l.responseHandler.extractResponseInfo(chatLog, &response)
	return &response, nil
}

// ChatCompletionStream handles streaming chat completion with SSE
func (l *ChatCompletionLogic) ChatCompletionStream() error {
	chatLog, processedPrompt, err := l.processRequest()
	msgs := l.request().Messages

	defer l.logCompletion(chatLog)

	if err == nil {
		msgs = processedPrompt.Messages
		chatLog.IsPromptProceed = true
	} else {
		err := fmt.Errorf("ChatCompletionStream failed to process request: %w", err)
		logger.Error("failed to process request in streaming", zap.Error(err))
		chatLog.AddError(types.ErrExtra, err)
		chatLog.IsPromptProceed = false
	}

	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLMEndpoint, l.request().Model, l.headers())
	if err != nil {
		l.responseHandler.sendSSEError(l.writer(), "LLM client creation failed", err)
		return fmt.Errorf("LLM client creation failed: %w", err)
	}

	// Get flusher for immediate response sending
	flusher, ok := l.writer().(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Variables to collect streaming response data
	var (
		responseContent strings.Builder
		finalUsage      *types.Usage
		modelStart      = time.Now()
	)

	// Stream completion using structured messages with raw response
	err = llmClient.ChatLLMWithMessagesStreamRaw(l.ctx, msgs, func(rawLine string) error {
		l.responseHandler.extractStreamingData(rawLine, &responseContent, &finalUsage)

		if !strings.HasPrefix(rawLine, "data: ") {
			rawLine = "data: " + rawLine
		}

		if _, err := fmt.Fprintf(l.writer(), "%s\n\n", rawLine); err != nil {
			logger.Error("SSE write failed", zap.Error(err))
			return err
		}
		flusher.Flush()
		return nil
	})

	if err != nil {
		l.responseHandler.sendSSEError(l.writer(), "Streaming completion failed", err)
		chatLog.AddError(types.ErrExtra, err)
		return fmt.Errorf("streaming completion failed: %w", err)
	}

	// Update chat log with completion info
	chatLog.MainModelLatency = time.Since(modelStart).Milliseconds()
	// Set response content and usage information
	chatLog.ResponseContent = responseContent.String()

	if finalUsage != nil {
		chatLog.Usage = *finalUsage
	} else {
		// Calculate usage if not provided in streaming response
		chatLog.Usage = l.responseHandler.
			calculateUsage(
				chatLog.CompressedTokens.All,
				chatLog.ResponseContent,
			)
		logger.Info("calculated usage for streaming response",
			zap.Int("totalTokens", chatLog.Usage.TotalTokens),
		)
	}

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
