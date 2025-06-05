package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/strategy"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"

	"github.com/google/uuid"
)

type ChatCompletionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatCompletionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatCompletionLogic {
	return &ChatCompletionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
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
	startTime := time.Now()
	requestID := uuid.New().String()

	// Initialize chat log
	chatLog := l.initializeChatLog(requestID, startTime, req)

	// Determine if compression is needed
	userMessageTokens := chatLog.OriginalTokens.UserTokens
	needsCompressUserMsg := l.svcCtx.Config.EnableCompression && userMessageTokens > l.svcCtx.Config.TokenThreshold
	log.Printf("[processRequest] userMessageTokens: %v, needsCompression: %v\n\n", userMessageTokens, needsCompressUserMsg)
	chatLog.CompressionTriggered = needsCompressUserMsg

	promptProcessor, err := strategy.NewCompressionProcessor(l.svcCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to new processor: %w", err)
	}

	processedPrompt, err := promptProcessor.ProcessPrompt(l.ctx, req, needsCompressUserMsg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process prompt: %w", err)
	}

	// Update chat log with processed prompt info
	l.updateChatLogWithProcessedPrompt(chatLog, processedPrompt, needsCompressUserMsg)

	return chatLog, processedPrompt, nil
}

// initializeChatLog creates and initializes a new ChatLog with basic information and original token stats
func (l *ChatCompletionLogic) initializeChatLog(requestID string, startTime time.Time, req *types.ChatCompletionRequest) *model.ChatLog {
	// Count original tokens
	userMessageTokens := l.countTokensInMessages(utils.GetUserMsgs(req.Messages))
	originalAllMessageTokens := l.countTokensInMessages(req.Messages)
	systemMessageTokens := originalAllMessageTokens - userMessageTokens

	return &model.ChatLog{
		RequestID:   requestID,
		Timestamp:   startTime,
		ClientID:    req.ClientId,
		ProjectPath: req.ProjectPath,
		Model:       req.Model,
		OriginalTokens: model.TokenStats{
			SystemTokens: systemMessageTokens,
			UserTokens:   userMessageTokens,
			All:          originalAllMessageTokens,
		},
		OriginalPrompt: req.Messages,
	}
}

// updateChatLogWithProcessedPrompt updates the chat log with information from the processed prompt
func (l *ChatCompletionLogic) updateChatLogWithProcessedPrompt(chatLog *model.ChatLog, processedPrompt *strategy.ProcessedPrompt, needsCompression bool) {
	// Record timing information from processed prompt
	chatLog.SemanticLatency = processedPrompt.SemanticLatency
	if needsCompression && processedPrompt.IsCompressed {
		chatLog.SummaryLatency = processedPrompt.SummaryLatency
	}

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
}

// ChatCompletion handles chat completion requests
func (l *ChatCompletionLogic) ChatCompletion() (resp *types.ChatCompletionResponse, err error) {
	chatLog, processedPrompt, err := l.processRequest(l.getRequest())
	if err != nil {
		return nil, err
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
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLMEndpoint, l.getRequest().Model)
	llmClient.SetHeaders(l.getHeaders())
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Generate completion using structured messages
	response, err := llmClient.ChatLLMWithMessagesRaw(l.ctx, processedPrompt.Messages)
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
	if err != nil {
		l.sendSSEError(l.getWriter(), "Failed to process request", err)
		return err
	}

	// Defer logging for streaming requests - will be called after streaming completes
	defer func() {
		chatLog.TotalLatency = time.Since(chatLog.Timestamp).Milliseconds()
		if l.svcCtx.LoggerService != nil {
			l.svcCtx.LoggerService.LogAsync(chatLog, l.getHeaders())
		}
	}()

	// Create LLM client for main model
	llmClient, err := client.NewLLMClient(l.svcCtx.Config.LLMEndpoint, l.getRequest().Model)
	llmClient.SetHeaders(l.getHeaders())
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

	// fmt.Printf("==> [ChatCompletionStream] processedPrompt: \n%v\n", processedPrompt.Messages)

	// Stream completion using structured messages with raw response
	err = llmClient.ChatLLMWithMessagesStreamRaw(l.ctx, processedPrompt.Messages, func(rawLine string) error {
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
				log.Printf("Failed to write SSE stream line: %v", writeErr)
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
		log.Printf("Calculated usage for streaming response - TotalTokens: %d", chatLog.Usage.TotalTokens)
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
	return utils.EstimateTokens(totalText)
}

func (l *ChatCompletionLogic) countTokens(text string) int {
	if l.svcCtx.TokenCounter != nil {
		return l.svcCtx.TokenCounter.CountTokens(text)
	}
	return utils.EstimateTokens(text)
}

// sendSSEError sends an error message in SSE format
func (l *ChatCompletionLogic) sendSSEError(w http.ResponseWriter, message string, err error) {
	log.Printf("%s: %v", message, err)

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
		log.Printf("Failed to marshal error response: %v", marshalErr)
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
	log.Printf("Extracting response info - Choices count: %d", len(response.Choices))

	// Extract response content from choices
	if len(response.Choices) > 0 {
		contentStr := utils.GetContentAsString(response.Choices[0].Message.Content)
		log.Printf("Response content length: %d", len(contentStr))
		chatLog.ResponseContent = contentStr
	}

	// Extract usage information
	log.Printf("Response usage - TotalTokens: %d, PromptTokens: %d, CompletionTokens: %d",
		response.Usage.TotalTokens, response.Usage.PromptTokens, response.Usage.CompletionTokens)

	if response.Usage.TotalTokens > 0 {
		chatLog.Usage = response.Usage
	} else {
		// Calculate usage if not provided
		chatLog.Usage = l.calculateUsage(chatLog.CompressedTokens.All, chatLog.ResponseContent)
		log.Printf("Calculated usage - TotalTokens: %d", chatLog.Usage.TotalTokens)
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
		log.Printf("Failed to parse streaming chunk: %v, data: %s", err, jsonData)
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
		log.Printf("Found usage information in streaming response")
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
		log.Printf("Extracted usage - PromptTokens: %d, CompletionTokens: %d, TotalTokens: %d",
			(*finalUsage).PromptTokens, (*finalUsage).CompletionTokens, (*finalUsage).TotalTokens)
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
