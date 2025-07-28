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
	"github.com/zgsm-ai/chat-rag/internal/functions"
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
	toolExecutor    functions.ToolExecutor
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
		toolExecutor:    svcCtx.ToolExecutor,
	}
}

const MaxToolCallDepth = 3

// processRequest handles common request processing logic
func (l *ChatCompletionLogic) processRequest() (*model.ChatLog, *ds.ProcessedPrompt, error) {
	logger.Info("starting to process request", zap.String("user", l.identity.UserName))
	startTime := time.Now()

	// Initialize chat log
	chatLog := l.newChatLog(startTime)

	promptArranger := promptflow.NewPromptProcessor(
		l.ctx,
		l.svcCtx,
		l.request.ExtraBody.PromptMode,
		l.headers,
		l.identity,
		l.request.Model,
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

	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLM, l.request.Model, l.headers)
	llmClient.SetTools(processedPrompt.Tools)
	if err != nil {
		l.responseHandler.sendSSEError(l.writer, err)
		chatLog.AddError(types.ErrServerError, err)
		return fmt.Errorf("LLM client creation failed: %w", err)
	}

	flusher, ok := l.writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// 主处理逻辑
	return l.handleStreamingWithTools(llmClient, flusher, msgs, chatLog, MaxToolCallDepth) // 最大递归深度3
}

func (l *ChatCompletionLogic) handleStreamingWithTools(
	llmClient client.LLMInterface,
	flusher http.Flusher,
	messages []types.Message,
	chatLog *model.ChatLog,
	remainingDepth int,
) error {
	logger.Info("starting to handle streaming with tools",
		zap.Int("remainingDepth", remainingDepth),
		zap.Int("MaxToolCallDepth", MaxToolCallDepth),
	)
	var (
		window       []string // sliding window buffer
		windowSize   = 15     // initial window size
		toolDetected bool     // whether tool is detected
		toolName     string   // tool name
		modelStart   = time.Now()
		finalUsage   *types.Usage
		response     *types.ChatCompletionResponse
		fullContent  strings.Builder // complete content record
		DONE         = "[DONE]"
	)

	// Phase 1: streaming processing
	err := llmClient.ChatLLMWithMessagesStreamRaw(l.ctx, messages, func(rawLine string) error {
		// 1. Parse content
		content, usage, resp := l.responseHandler.extractStreamingData(rawLine)
		finalUsage = usage
		if resp != nil {
			response = resp
		}
		if content == "" {
			if err := l.sendRawLine(flusher, rawLine); err != nil {
				return err
			}
			return nil
		}

		// Add to window and complete content
		window = append(window, content)
		if content != DONE {
			fullContent.WriteString(content)
		}

		// 3. Tool detection (only when tool not yet detected)
		if !toolDetected && l.toolExecutor != nil && remainingDepth > 0 {
			currentContent := strings.Join(window, "")
			hasTool, name := l.toolExecutor.DetectTools(l.ctx, currentContent)

			if hasTool {
				toolDetected = true
				toolName = name
				logger.Info("detected server xml tool", zap.String("name", name))

				// Send content before tool call
				toolStartIndex := strings.Index(currentContent, "<"+toolName+">")
				if toolStartIndex > 0 {
					preToolContent := currentContent[:toolStartIndex]
					if err := l.sendStreamContent(flusher, response, preToolContent); err != nil {
						logger.Error("failed to sendStreamContent when detecting tool",
							zap.String("preToolContent", preToolContent), zap.Error(err))
						return err
					}
				}

				window = []string{currentContent[toolStartIndex:]}
			}
		}

		// 4. Send content beyond window
		if !toolDetected && len(window) >= windowSize {
			if err := l.sendStreamContent(flusher, response, window[0]); err != nil {
				return err
			}
			window = window[1:]
		}

		return nil
	})

	if err != nil {
		logger.Error("ChatLLMWithMessagesStreamRaw error", zap.Error(err))
		if l.isContextLengthError(err) {
			logger.Error("Input context too long", zap.Error(err))
			lengthErr := types.NewContextTooLongError()
			l.responseHandler.sendSSEError(l.writer, lengthErr)
			chatLog.AddError(types.ErrContextExceeded, lengthErr)
			return nil
		}

		l.responseHandler.sendSSEError(l.writer, err)
		chatLog.AddError(types.ErrApiError, err)
		return nil
	}

	// Phase 2: Handle tool call (if detected)
	if toolDetected {
		// Execute tool (using all collected content)
		toolContent := strings.Join(window, "")
		logger.Info("starting to call tool", zap.String("name", toolName))
		if err := l.sendStreamContent(flusher, response, "<tool>Starting to execute tool...\n</tool>"); err != nil {
			return err
		}
		newMessages, err := l.toolExecutor.ExecuteTools(
			l.ctx,
			toolName,
			toolContent,
			messages,
		)
		if err != nil {
			return err
		}

		// // Send tool call end event
		// TODO send tool call end event
		if err := l.sendStreamContent(flusher, response, "<tool>Tool call completed</tool>"); err != nil {
			return err
		}

		// Recursive processing
		return l.handleStreamingWithTools(
			llmClient,
			flusher,
			newMessages,
			chatLog,
			remainingDepth-1,
		)
	}

	// Phase 3: Send remaining content when no tool call
	logger.Info("starting to send remaining content before ending.")
	if len(window) > 0 {
		if window[len(window)-1] == DONE {
			window = window[:len(window)-1]
		}

		endContent := strings.Join(window, "")
		if err := l.sendStreamContent(flusher, response, endContent); err != nil {
			return err
		}
		if err := l.sendRawLine(flusher, DONE); err != nil {
			return err
		}
	}

	// Update statistics
	if remainingDepth == 3 {
		chatLog.MainModelLatency = time.Since(modelStart).Milliseconds()
		chatLog.ResponseContent = fullContent.String()

		if finalUsage != nil {
			chatLog.Usage = *finalUsage
		} else {
			chatLog.Usage = l.responseHandler.calculateUsage(
				chatLog.CompressedTokens.All,
				chatLog.ResponseContent,
			)
			logger.Info("calculated usage for streaming response",
				zap.Int("totalTokens", chatLog.Usage.TotalTokens),
			)
		}
	}

	return nil
}

func (l *ChatCompletionLogic) sendRawLine(flusher http.Flusher, raw string) error {
	if !strings.HasPrefix(raw, "data: ") {
		raw = "data: " + raw
	}
	_, err := fmt.Fprintf(l.writer, "%s\n\n", raw)
	flusher.Flush()
	return err
}

func (l *ChatCompletionLogic) sendStreamContent(flusher http.Flusher, response *types.ChatCompletionResponse, content string) error {
	if response == nil {
		logger.Warn("response is nil, use default response", zap.String("method", "sendStreamContent"))
		response = &types.ChatCompletionResponse{}
	}

	response.Choices = []types.Choice{{
		Delta: types.Delta{
			Content: content,
		},
	}}
	jsonData, _ := json.Marshal(response)
	_, err := fmt.Fprintf(l.writer, "data: %s\n\n", jsonData)
	flusher.Flush()
	return err
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
