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

func (c *LLMClient) GetModelName() string {
	return c.modelName
}

// GenerateContent generate content using a structured message format
func (c *LLMClient) GenerateContent(ctx context.Context, systemPrompt string, userMessages []types.Message) (string, error) {
	// Create a new slice of messages for the summary request
	var messages []types.Message

	// Add system message with the summary prompt
	messages = append(messages, types.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	messages = append(messages, userMessages...)

	// fmt.Printf("==> [GenerateContent] messages:\n %v\n\n", messages)
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
	// fmt.Printf("==> [GenerateContent] content:\n %v \n\n", content)
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
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "...(truncated)"
		}
		return fmt.Errorf("API request failed with status %d, response body: %s", resp.StatusCode, bodyStr)
	}

	// 逐行读取流式响应
	scanner := bufio.NewScanner(resp.Body)
	// 增加缓冲区大小以处理较长的响应行
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// 调试输出
		// fmt.Printf("Received line: %s\n", line)

		// 处理非空行，包括空的data行
		if line != "" || strings.HasPrefix(line, "data:") {
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

	// 记录请求数据用于调试
	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// 创建请求
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

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "...(truncated)"
		}
		return nil_resp, fmt.Errorf("API request failed with status %d, response body: %s", resp.StatusCode, bodyStr)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to read response body: %w", err)
	}

	var result types.ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "...(truncated)"
		}
		return nil_resp, fmt.Errorf("failed to parse response (invalid JSON? body: %s): %w", bodyStr, err)
	}

	return result, nil
}
