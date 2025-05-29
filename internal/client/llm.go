package client

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpc"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

// LLMClient handles communication with language models
type LLMClient struct {
	model      llms.Model
	modelName  string
	endpoint   string
	token      string
	httpClient httpc.Service
}

// NewLLMClient creates a new LLM client instance
func NewLLMClient(endpoint string, modelName string) (*LLMClient, error) {
	// Check for empty endpoint
	if endpoint == "" {
		return nil, fmt.Errorf("NewLLMClient endpoint cannot be empty")
	}

	// Create model client
	model, err := openai.New(
		openai.WithBaseURL(endpoint),
		openai.WithModel(modelName),
		openai.WithToken("default"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create model client: %w", err)
	}

	// Create HTTP client
	httpClient := httpc.NewService("llm-client")

	return &LLMClient{
		model:      model,
		modelName:  modelName,
		endpoint:   endpoint,
		token:      "default",
		httpClient: httpClient,
	}, nil
}

// GenerateCompletion generates completion using the main model
func (c *LLMClient) GenerateCompletion(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	response, err := c.model.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}, options...)

	if err != nil {
		return "", fmt.Errorf("failed to generate completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no completion generated")
	}

	return response.Choices[0].Content, nil
}

// GenerateCompletionStream generates streaming completion using the main model
func (c *LLMClient) GenerateCompletionStream(ctx context.Context, prompt string, callback func(string) error, options ...llms.CallOption) error {
	_, err := c.model.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}, append(options, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		return callback(string(chunk))
	}))...)

	return err
}

// ChatLLMWithMessages generate content using a structured message format
func (c *LLMClient) ChatLLMWithMessages(ctx context.Context, messages []types.Message) (string, error) {
	// Convert types.Message to langchaingo message format
	var langchainMessages []llms.MessageContent

	for _, msg := range messages {
		var msgType llms.ChatMessageType

		switch msg.Role {
		case "system":
			msgType = llms.ChatMessageTypeSystem
		case "assistant":
			msgType = llms.ChatMessageTypeAI
		case "user":
			msgType = llms.ChatMessageTypeHuman
		default:
			msgType = llms.ChatMessageTypeHuman
		}

		langchainMessages = append(langchainMessages, llms.TextParts(msgType, utils.GetContentAsString(msg.Content)))
	}

	// Generate using structured messages
	response, err := c.model.GenerateContent(ctx, langchainMessages)

	if err != nil {
		logx.Errorw("failed to generate content with messages", logx.Field("error", err))
		return "", fmt.Errorf("failed to generate content with messages: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	return response.Choices[0].Content, nil
}

// ChatLLMWithMessagesStream generates streaming content using structured message format
func (c *LLMClient) ChatLLMWithMessagesStream(ctx context.Context, messages []types.Message, callback func(string) error) error {
	// Convert types.Message to langchaingo message format
	var langchainMessages []llms.MessageContent

	for _, msg := range messages {
		var msgType llms.ChatMessageType

		switch msg.Role {
		case "system":
			msgType = llms.ChatMessageTypeSystem
		case "assistant":
			msgType = llms.ChatMessageTypeAI
		case "user":
			msgType = llms.ChatMessageTypeHuman
		default:
			msgType = llms.ChatMessageTypeHuman
		}

		langchainMessages = append(langchainMessages, llms.TextParts(msgType, utils.GetContentAsString(msg.Content)))
	}

	// Generate streaming content using structured messages
	// Note: We'll handle usage calculation manually since langchaingo may not support stream_options directly
	_, err := c.model.GenerateContent(ctx, langchainMessages,
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Println("Received chunk:", string(chunk))
			return callback(string(chunk))
		}),
	)

	if err != nil {
		logx.Errorw("failed to generate streaming content with messages", logx.Field("error", err))
		return fmt.Errorf("failed to generate streaming content with messages: %w", err)
	}

	return nil
}

// CountTokens estimates token count for a given text
func (c *LLMClient) CountTokens(text string) int {
	// Simple token estimation: roughly 4 characters per token
	// This is a rough approximation, for production use a proper tokenizer
	return len(text) / 4
}

// ChatLLMWithMessagesStreamRaw 使用go-zero HTTP客户端直接调用接口获取流式响应的原始数据
func (c *LLMClient) ChatLLMWithMessagesStreamRaw(ctx context.Context, messages []types.Message, headers http.Header, callback func(string) error) error {
	// 准备请求数据结构体
	requestPayload := types.ChatLLMRequestStream{
		Model:    c.modelName,
		Messages: messages,
		Stream:   true,
		StreamOptions: types.StreamOptions{
			IncludeUsage: true,
		},
	}

	// 构建完整的URL
	url := strings.TrimSuffix(c.endpoint, "/") + "/chat/completions"

	// 创建临时的HTTP客户端，透传请求头
	tempClient := httpc.NewService("llm-client", func(r *http.Request) *http.Request {
		// 合并请求头而不是直接替换
		for key, values := range headers {
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}
		return r
	})

	// 使用临时客户端发送流式请求
	resp, err := tempClient.Do(ctx, "POST", url, requestPayload)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// 逐行读取流式响应
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// 直接返回原始行数据，包括 "data: " 前缀
		if line != "" {
			if err := callback(line); err != nil {
				return fmt.Errorf("callback error: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	return nil
}

// ChatLLMWithMessagesRaw 使用go-zero HTTP客户端直接调用接口获取非流式响应的原始数据
func (c *LLMClient) ChatLLMWithMessagesRaw(ctx context.Context, messages []types.Message, headers http.Header) (types.ChatCompletionResponse, error) {
	// 准备请求数据结构体
	requestPayload := types.ChatLLMRequest{
		Model:    c.modelName,
		Messages: messages,
	}

	nil_resp := types.ChatCompletionResponse{}

	// 构建完整的URL
	url := strings.TrimSuffix(c.endpoint, "/") + "/chat/completions"

	// 创建临时的HTTP客户端，透传请求头
	tempClient := httpc.NewService("llm-client", func(r *http.Request) *http.Request {
		// 合并请求头
		for key, values := range headers {
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}
		return r
	})

	resp, err := tempClient.Do(ctx, http.MethodPost, url, requestPayload)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil_resp, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// 读取响应体并直接返回原始JSON
	var result types.ChatCompletionResponse
	err = httpc.ParseJsonBody(resp, &result)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
