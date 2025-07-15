package logic

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/promptflow"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/ds"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

type ChatCompletionLogic struct {
	ctx             context.Context
	svcCtx          *bootstrap.ServiceContext
	request         *types.ChatCompletionRequest
	writer          http.ResponseWriter
	headers         *http.Header
	identity        *model.Identity
	responseHandler *ResponseHandler
}

func NewChatCompletionLogic(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	request *types.ChatCompletionRequest,
	writer http.ResponseWriter,
	headers *http.Header,
	identity *model.Identity,
) *ChatCompletionLogic {
	return &ChatCompletionLogic{
		ctx:             ctx,
		svcCtx:          svcCtx,
		identity:        identity,
		responseHandler: NewResponseHandler(ctx, svcCtx),
		request:         request,
		writer:          writer,
		headers:         headers,
	}
}

// processRequest handles common request processing logic
func (l *ChatCompletionLogic) processRequest() (*model.ChatLog, *ds.ProcessedPrompt, error) {
	logger.Info("start to process request", zap.String("user", l.identity.UserName))
	startTime := time.Now()

	// Initialize chat log
	chatLog := l.newChatLog(startTime)

	promptArranger := promptflow.NewPromptProcessor(
		l.ctx,
		l.svcCtx,
		l.request.ExtraBody.PromptMode,
		l.headers,
		l.identity,
	)
	processedPrompt, err := promptArranger.Arrange(l.request.Messages)
	if err != nil {
		return chatLog, nil, fmt.Errorf("failed to process prompt:\n %w", err)
	}

	// Update chat log with processed prompt info
	l.updateChatLog(chatLog, processedPrompt)
	return chatLog, processedPrompt, nil
}

func (l *ChatCompletionLogic) newChatLog(startTime time.Time) *model.ChatLog {
	userTokens := l.countTokensInMessages(utils.GetUserMsgs(l.request.Messages))
	allTokens := l.countTokensInMessages(l.request.Messages)

	return &model.ChatLog{
		Identity:   *l.identity,
		Timestamp:  startTime,
		Model:      l.request.Model,
		PromptMode: string(l.request.ExtraBody.PromptMode),
		OriginalTokens: model.TokenStats{
			SystemTokens: allTokens - userTokens,
			UserTokens:   userTokens,
			All:          allTokens,
		},
		OriginalPrompt: l.request.Messages,
	}
}

// updateChatLog updates the chat log with information from the processed prompt
func (l *ChatCompletionLogic) updateChatLog(chatLog *model.ChatLog, processedPrompt *ds.ProcessedPrompt) {
	// Record timing information from processed prompt
	chatLog.SemanticLatency = processedPrompt.SemanticLatency
	chatLog.SummaryLatency = processedPrompt.SummaryLatency
	chatLog.SemanticContext = processedPrompt.SemanticContext

	// Update log with processed prompt info
	chatLog.IsUserPromptCompressed = processedPrompt.IsUserPromptCompressed
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
		l.svcCtx.LoggerService.LogAsync(chatLog, l.headers)
	}
}

// ChatCompletion handles chat completion requests
func (l *ChatCompletionLogic) ChatCompletion() (resp *types.ChatCompletionResponse, err error) {
	chatLog, processedPrompt, err := l.processRequest()
	msgs := l.request.Messages

	defer l.logCompletion(chatLog)

	if err == nil {
		msgs = processedPrompt.Messages
		chatLog.IsPromptProceed = true
	} else {
		err := fmt.Errorf("ChatCompletion failed to process request:\n%w", err)
		logger.Error("failed to process request", zap.Error(err))
		chatLog.AddError(types.ErrServerError, err)
		chatLog.IsPromptProceed = false
	}

	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLM, l.request.Model, l.headers)
	if err != nil {
		chatLog.AddError(types.ErrServerError, err)
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	modelStart := time.Now()
	// Generate completion using structured messages
	response, err := llmClient.ChatLLMWithMessagesRaw(l.ctx, msgs)
	if err != nil {
		if l.isContextLengthError(err) {
			logger.Error("Input context too long, exceeded limit.", zap.Error(err))
			lengthErr := types.NewContextTooLongError()
			l.responseHandler.sendSSEError(l.writer, lengthErr)
			chatLog.AddError(types.ErrContextExceeded, lengthErr)
			return nil, lengthErr
		}

		chatLog.AddError(types.ErrApiError, err)
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
	msgs := l.request.Messages

	defer l.logCompletion(chatLog)

	if err == nil {
		msgs = processedPrompt.Messages
		chatLog.IsPromptProceed = true
	} else {
		err := fmt.Errorf("ChatCompletionStream failed to process request: %w", err)
		logger.Error("failed to process request in streaming", zap.Error(err))
		chatLog.AddError(types.ErrServerError, err)
		chatLog.IsPromptProceed = false
	}

	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLM, l.request.Model, l.headers)
	if err != nil {
		l.responseHandler.sendSSEError(l.writer, err)
		chatLog.AddError(types.ErrServerError, err)
		return fmt.Errorf("LLM client creation failed: %w", err)
	}

	// Get flusher for immediate response sending
	flusher, ok := l.writer.(http.Flusher)
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

		// Remove XML tags from rawLine
		rawLine = regexp.MustCompile(`assistant`).ReplaceAllString(rawLine, "function")
		rawLine = regexp.MustCompile(`<`).ReplaceAllString(rawLine, "")
		rawLine = regexp.MustCompile(`>`).ReplaceAllString(rawLine, "")
		rawLine = regexp.MustCompile(`</`).ReplaceAllString(rawLine, "")
		fmt.Printf("==> %s", rawLine)

		if !strings.HasPrefix(rawLine, "data: ") {
			rawLine = "data: " + rawLine
		}

		if _, err := fmt.Fprintf(l.writer, "%s\n\n", rawLine); err != nil {
			logger.Error("SSE write failed", zap.Error(err))
			chatLog.AddError(types.ErrServerError, err)
			return err
		}
		flusher.Flush()
		return nil
	})

	if err != nil {
		if l.isContextLengthError(err) {
			logger.Error("Input context too long, exceeded limit.", zap.Error(err))
			lengthErr := types.NewContextTooLongError()
			l.responseHandler.sendSSEError(l.writer, lengthErr)
			chatLog.AddError(types.ErrContextExceeded, lengthErr)
			return nil
		}

		l.responseHandler.sendSSEError(l.writer, err)
		chatLog.AddError(types.ErrApiError, err)
		return nil
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

// isContextLengthError checks if the error is due to context length exceeded
func (l *ChatCompletionLogic) isContextLengthError(err error) bool {
	errMsg := err.Error()
	return strings.Contains(errMsg, "This model's maximum context length") ||
		strings.Contains(errMsg, "Input text is too long")
}

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
