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
	usage           *types.Usage
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

const MaxToolCallDepth = 10

// processRequest handles common request processing logic
func (l *ChatCompletionLogic) processRequest() (*model.ChatLog, *ds.ProcessedPrompt, error) {
	logger.InfoC(l.ctx, "starting to process request", zap.String("user", l.identity.UserName))
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
		return nil, err
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

	logger.Info("start to handle streaming response ...")
	return l.handleStreamingWithTools(llmClient, flusher, msgs, chatLog, MaxToolCallDepth)
}

// streamState holds the state for streaming processing
type streamState struct {
	window       []string // Window of streamed content used for detect tools
	windowSize   int
	toolDetected bool
	toolName     string
	fullContent  strings.Builder
	response     *types.ChatCompletionResponse
	modelStart   time.Time
}

func newStreamState() *streamState {
	return &streamState{
		windowSize: 6,
		modelStart: time.Now(),
	}
}

func (l *ChatCompletionLogic) handleStreamingWithTools(
	llmClient client.LLMInterface,
	flusher http.Flusher,
	messages []types.Message,
	chatLog *model.ChatLog,
	remainingDepth int,
) error {
	logger.InfoC(l.ctx, "starting to handle streaming with tools",
		zap.Int("remainingDepth", remainingDepth),
		zap.Int("MaxToolCallDepth", MaxToolCallDepth),
	)

	state := newStreamState()

	// Phase 1: Process streaming response
	toolDetected, err := l.processStream(llmClient, flusher, messages, state, remainingDepth)
	if err != nil {
		return l.handleStreamError(err, chatLog)
	}

	// Phase 2: Handle tool execution or complete response
	if toolDetected {
		return l.handleToolExecution(llmClient, flusher, messages, chatLog, state, remainingDepth)
	}

	return l.completeStreamResponse(flusher, chatLog, state)
}

// processStream handles the streaming response processing
func (l *ChatCompletionLogic) processStream(
	llmClient client.LLMInterface,
	flusher http.Flusher,
	messages []types.Message,
	state *streamState,
	remainingDepth int,
) (bool, error) {
	err := llmClient.ChatLLMWithMessagesStreamRaw(l.ctx, messages, func(llmResp client.LLMResponse) error {
		l.handleResonseHeaders(llmResp.Header, []string{
			types.HeaderUserInput,
			types.HeaderSelectLLm,
		})

		return l.handleStreamChunk(flusher, llmResp.ResonseLine, state, remainingDepth)
	})

	return state.toolDetected, err
}

// handleResonseHeaders Set the specified request header to the response
func (l *ChatCompletionLogic) handleResonseHeaders(header *http.Header, requiredHeaders []string) {
	for _, headerName := range requiredHeaders {
		if headerValue := header.Get(headerName); headerValue != "" {
			if l.writer.Header().Get(headerName) != "" {
				continue
			}

			l.writer.Header().Set(headerName, headerValue)
			logger.InfoC(l.ctx, "Response header setted",
				zap.String("header", headerName), zap.String("value", headerValue))
		}
	}
}

// handleStreamChunk processes individual streaming chunks
func (l *ChatCompletionLogic) handleStreamChunk(
	flusher http.Flusher,
	rawLine string,
	state *streamState,
	remainingDepth int,
) error {
	content, usage, resp := l.responseHandler.extractStreamingData(rawLine)
	// DEBUG
	// fmt.Println(content)
	if resp != nil {
		state.response = resp
	}
	if usage != nil {
		l.usage = usage
	}
	if content == "" {
		return l.sendRawLine(flusher, rawLine)
	}

	// Add to window and complete content
	state.window = append(state.window, content)
	if content != "[DONE]" {
		state.fullContent.WriteString(content)
	}

	// Check for tool detection
	if !state.toolDetected && l.toolExecutor != nil && remainingDepth > 0 {
		if err := l.detectAndHandleTool(flusher, state); err != nil {
			return err
		}
	}

	// Send content beyond window
	if !state.toolDetected && len(state.window) >= state.windowSize {
		if err := l.sendStreamContent(flusher, state.response, state.window[0]); err != nil {
			return err
		}
		state.window = state.window[1:]
	}

	return nil
}

// detectAndHandleTool handles tool detection and pre-tool content sending
func (l *ChatCompletionLogic) detectAndHandleTool(flusher http.Flusher, state *streamState) error {
	currentContent := strings.Join(state.window, "")
	hasTool, name := l.toolExecutor.DetectTools(l.ctx, currentContent)

	if !hasTool {
		return nil
	}

	state.toolDetected = true
	state.toolName = name
	logger.Info("detected server xml tool", zap.String("name", name))

	// Send content before tool call
	toolStartIndex := strings.Index(currentContent, "<"+name+">")
	if toolStartIndex > 0 {
		preToolContent := currentContent[:toolStartIndex]
		if err := l.sendStreamContent(flusher, state.response, preToolContent); err != nil {
			logger.Error("failed to sendStreamContent when detecting tool",
				zap.String("preToolContent", preToolContent), zap.Error(err))
			return err
		}
	}

	state.window = []string{currentContent[toolStartIndex:]}
	return nil
}

// handleToolExecution executes the detected tool and continues processing
func (l *ChatCompletionLogic) handleToolExecution(
	llmClient client.LLMInterface,
	flusher http.Flusher,
	messages []types.Message,
	chatLog *model.ChatLog,
	state *streamState,
	remainingDepth int,
) error {
	logger.Info("starting to call tool", zap.String("name", state.toolName))
	toolContent := strings.Join(state.window, "")
	toolCall := model.ToolCall{
		ToolName:  state.toolName,
		ToolInput: toolContent,
	}

	l.updateToolStatus(state.toolName, types.ToolStatusRunning)
	// Send tool use information to client page
	if err := l.sendStreamContent(flusher, state.response,
		fmt.Sprintf("%s`%s` %s", types.StrFilterToolSearchStart, state.toolName,
			types.StrFilterToolSearchEnd)); err != nil {
		return err
	}

	// wait client to refesh content
	for i := 0; i < 5; i++ {
		if err := l.sendStreamContent(flusher, state.response, "."); err != nil {
			return err
		}
		time.Sleep(600 * time.Millisecond)
	}

	// execute and record tool call latency
	toolStart := time.Now()
	result, err := l.toolExecutor.ExecuteTools(l.ctx, state.toolName, toolContent)
	toolLatency := time.Since(toolStart).Milliseconds()
	toolCall.Latency = toolLatency
	toolCall.ToolOutput = result

	status := types.ToolStatusSuccess
	if err != nil {
		logger.WarnC(l.ctx, "tool execute failed", zap.String("tool", state.toolName), zap.Error(err))
		status = types.ToolStatusFailed
		result = fmt.Sprintf("%s execute failed, err: %v", state.toolName, err)
		toolCall.Error = err.Error()
	} else {
		logResult := result
		if len(logResult) > 400 {
			logResult = logResult[:400] + "..."
		}
		logger.InfoC(l.ctx, "tool execute succeed", zap.String("tool", state.toolName),
			zap.String("result", logResult))
	}
	toolCall.ResultStatus = string(status)

	messages = append(messages,
		types.Message{
			Role:    types.RoleAssistant,
			Content: state.fullContent.String(),
		},
		types.Message{
			Role: types.RoleUser,
			Content: []model.Content{
				{
					Type: model.ContTypeText,
					Text: fmt.Sprintf("[%s] Result:", state.toolName),
				}, {
					Type: model.ContTypeText,
					Text: result,
				}, {
					Type: model.ContTypeText,
					Text: fmt.Sprintf("Please summarize the key findings and/or code from the results above within the <thinking></thinking> tags. No need to summarize error messages. \nIf the search failed, don't say 'failed', describe this outcome as 'did not found relevant results' instead - MUST NOT using terms like 'failure', 'error', or 'unsuccessful' in your description. \nIn your summary, must include the name of the tool used and specify which tools you intend to use next. \nWhen appropriate, prioritize using these tools: %s", l.toolExecutor.GetAllTools()),
				},
			},
		},
	)

	l.updateToolStatus(state.toolName, status)
	chatLog.CompressedPrompt = messages
	chatLog.ToolCalls = append(chatLog.ToolCalls, toolCall)

	// sending tool call ending response to client page
	if err := l.sendStreamContent(flusher, state.response, types.StrFilterToolAnalyzing); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := l.sendStreamContent(flusher, state.response, "."); err != nil {
			return err
		}
	}
	if err := l.sendStreamContent(flusher, state.response, "\n"); err != nil {
		return err
	}

	// Recursive processing
	return l.handleStreamingWithTools(
		llmClient,
		flusher,
		messages,
		chatLog,
		remainingDepth-1,
	)
}

// completeStreamResponse sends remaining content and updates statistics
func (l *ChatCompletionLogic) completeStreamResponse(
	flusher http.Flusher,
	chatLog *model.ChatLog,
	state *streamState,
) error {
	logger.Info("starting to send remaining content before ending.")

	if len(state.window) > 0 {
		if state.window[len(state.window)-1] == "[DONE]" {
			state.window = state.window[:len(state.window)-1]
		}

		endContent := strings.Join(state.window, "")

		if state.response == nil {
			logger.WarnC(l.ctx, "state.response is nil when sending remaining content")
			state.response = &types.ChatCompletionResponse{}
		}

		if l.usage != nil {
			state.response.Usage = *l.usage
		} else {
			logger.WarnC(l.ctx, "usage is nil when content ending")
		}

		if err := l.sendStreamContent(flusher, state.response, endContent); err != nil {
			return err
		}

		if err := l.sendRawLine(flusher, "[DONE]"); err != nil {
			return err
		}
	}

	l.updateStreamStats(chatLog, state)

	return nil
}

// handleStreamError handles streaming errors with appropriate error responses
func (l *ChatCompletionLogic) handleStreamError(err error, chatLog *model.ChatLog) error {
	logger.ErrorC(l.ctx, "ChatLLMWithMessagesStreamRaw error", zap.Error(err))

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

// updateStreamStats updates chat log with streaming statistics
func (l *ChatCompletionLogic) updateStreamStats(chatLog *model.ChatLog, state *streamState) {
	chatLog.MainModelLatency = time.Since(state.modelStart).Milliseconds()
	chatLog.ResponseContent = state.fullContent.String()

	if l.usage != nil {
		chatLog.Usage = *l.usage
	} else {
		chatLog.Usage = l.responseHandler.calculateUsage(
			chatLog.CompressedTokens.All,
			chatLog.ResponseContent,
		)
		logger.Info("calculated usage for streaming response")
	}

	logger.Info("prompt usage", zap.Any("usage", chatLog.Usage))
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

func (l *ChatCompletionLogic) updateToolStatus(toolName string, status types.ToolStatus) {
	if l.identity.RequestID == "" {
		logger.Warn("requestID is empty, skip updating tool status")
		return
	}
	toolStatusKey := types.ToolStatusRedisKeyPrefix + l.identity.RequestID

	if err := l.svcCtx.RedisClient.SetHashField(l.ctx, toolStatusKey, toolName, string(status), 5*time.Minute); err != nil {
		logger.Error("failed to update tool status in redis",
			zap.String("toolName", toolName),
			zap.String("status", string(status)),
			zap.Error(err))
	}

	logger.Info("Tool execute status updated", zap.String("tool", toolName),
		zap.String("execute status", string(status)))
}

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
