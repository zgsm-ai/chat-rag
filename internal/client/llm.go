package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

// LLMInterface defines the interface for LLM clients
type LLMInterface interface {
	// GetModelName returns the name of the model
	GetModelName() string
	// GenerateContent directly generates non-streaming content with system prompts and user prompts
	GenerateContent(ctx context.Context, systemPrompt string, userMessages []types.Message) (string, error)
	// ChatLLMWithMessagesStreamRaw directly calls the API using HTTP client to get raw streaming response
	ChatLLMWithMessagesStreamRaw(ctx context.Context, messages []types.Message, callback func(string) error) error
	//ChatLLMWithMessagesRaw directly calls the API using HTTP client to get raw non-streaming response
	ChatLLMWithMessagesRaw(ctx context.Context, messages []types.Message) (types.ChatCompletionResponse, error)
}

// LLMClient handles communication with language models
type LLMClient struct {
	modelName  string
	endpoint   string
	funcModels []string
	headers    *http.Header
	httpClient *http.Client
}

// NewLLMClient creates a new LLM client instance
func NewLLMClient(config config.LLMConfig, modelName string, headers *http.Header) (LLMInterface, error) {
	// Check for empty endpoint
	if config.Endpoint == "" || headers == nil {
		return nil, fmt.Errorf("NewLLMClient llmEndpoint cannot be empty")
	}

	// Create HTTP client
	httpClient := &http.Client{}

	return &LLMClient{
		modelName:  modelName,
		endpoint:   config.Endpoint,
		funcModels: config.FuncCallingModels,
		httpClient: httpClient,
		headers:    headers,
	}, nil
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
		Role:    types.RoleSystem,
		Content: systemPrompt,
	})

	messages = append(messages, userMessages...)
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

// ChatLLMWithMessagesStreamRaw directly calls the API using HTTP client to get raw streaming response
func (c *LLMClient) ChatLLMWithMessagesStreamRaw(ctx context.Context, messages []types.Message, callback func(string) error) error {
	if callback == nil {
		return fmt.Errorf("callback function cannot be nil")
	}

	// Prepare request data structure
	requestPayload := types.ChatLLMRequestStream{
		Model:    c.modelName,
		Messages: messages,
		Stream:   true,
		StreamOptions: types.StreamOptions{
			IncludeUsage: true,
		},
	}

	// Create request
	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	reader := strings.NewReader(string(jsonData))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set request headers
	for key, values := range *c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Ensure Content-Length is set correctly
	req.ContentLength = int64(reader.Len())

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		logger.Warn("LLMClient get straming error response",
			zap.Int("status code", resp.StatusCode),
			zap.String("body", bodyStr),
		)
		return fmt.Errorf("%s", bodyStr)
	}

	// Read streaming response line by line
	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer size to handle long response lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Arrange non-empty lines, including empty data lines
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

// ChatLLMWithMessagesRaw directly calls the API using HTTP client to get raw non-streaming response
func (c *LLMClient) ChatLLMWithMessagesRaw(ctx context.Context, messages []types.Message) (types.ChatCompletionResponse, error) {
	// Prepare request data structure
	requestPayload := types.ChatLLMRequest{
		Model:    c.modelName,
		Messages: messages,
	}

	nil_resp := types.ChatCompletionResponse{}

	// Log request data for debugging
	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Create request
	reader := strings.NewReader(string(jsonData))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, reader)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to create request: %w", err)
	}

	// Set request headers
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for key, values := range *c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Ensure Content-Length is set correctly
	req.ContentLength = int64(reader.Len())

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		logger.Warn("LLMClient get error response",
			zap.Int("status code", resp.StatusCode),
			zap.String("body", bodyStr),
		)
		return nil_resp, fmt.Errorf("%s", bodyStr)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil_resp, fmt.Errorf("failed to read response body: %w", err)
	}

	var result types.ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		bodyStr := string(body)
		return nil_resp, fmt.Errorf("failed to parse response (invalid JSON? body: %s)\nerror: %w", bodyStr, err)
	}

	return result, nil
}
