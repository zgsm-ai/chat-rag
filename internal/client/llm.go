package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

// LLMClient handles communication with language models
type LLMClient struct {
	modelName  string
	endpoint   string
	headers    *http.Header
	httpClient *http.Client
}

// NewLLMClient creates a new LLM client instance
func NewLLMClient(endpoint string, modelName string) (*LLMClient, error) {
	// Check for empty endpoint
	if endpoint == "" {
		return nil, fmt.Errorf("NewLLMClient endpoint cannot be empty")
	}

	// Create HTTP client
	httpClient := &http.Client{}

	return &LLMClient{
		modelName:  modelName,
		endpoint:   endpoint,
		httpClient: httpClient,
	}, nil
}

func (c *LLMClient) SetHeaders(headers *http.Header) {
	c.headers = headers
}

// GenerateContent generate content using a structured message format
func (c *LLMClient) GenerateContent(ctx context.Context, messages []types.Message) (string, error) {
	// Call ChatLLMWithMessagesRaw to get the raw response
	result, err := c.ChatLLMWithMessagesRaw(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("failed to get response from ChatLLMWithMessagesRaw: %w", err)
	}

	// Check if there are any choices in the response
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	// Extract content from the first choice's message
	content := utils.GetContentAsString(result.Choices[0].Message.Content)
	return content, nil
}

// ChatLLMWithMessagesStreamRaw 使用HTTP客户端直接调用接口获取流式响应的原始数据
func (c *LLMClient) ChatLLMWithMessagesStreamRaw(ctx context.Context, messages []types.Message, callback func(string) error) error {
	// 准备请求数据结构体
	requestPayload := types.ChatLLMRequestStream{
		Model:    c.modelName,
		Messages: messages,
		Stream:   true,
		StreamOptions: types.StreamOptions{
			IncludeUsage: true,
		},
	}

	// 创建请求
	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	reader := strings.NewReader(string(jsonData))
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for key, values := range *c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 确保Content-Length正确设置
	req.ContentLength = int64(reader.Len())

	// 发送请求
	resp, err := c.httpClient.Do(req)
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

// ChatLLMWithMessagesRaw 使用HTTP客户端直接调用接口获取非流式响应的原始数据
func (c *LLMClient) ChatLLMWithMessagesRaw(ctx context.Context, messages []types.Message) (types.ChatCompletionResponse, error) {
	// 准备请求数据结构体
	requestPayload := types.ChatLLMRequest{
		Model:    c.modelName,
		Messages: messages,
	}

	nil_resp := types.ChatCompletionResponse{}

	// 创建请求
	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	reader := strings.NewReader(string(jsonData))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, reader)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for key, values := range *c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 确保Content-Length正确设置
	req.ContentLength = int64(reader.Len())

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil_resp, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// 读取响应体并直接返回原始JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to read response body: %w", err)
	}

	var result types.ChatCompletionResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
