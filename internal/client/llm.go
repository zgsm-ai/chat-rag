package client

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

// LLMClient handles communication with language models
type LLMClient struct {
	mainModel    llms.Model
	summaryModel llms.Model
}

// NewLLMClient creates a new LLM client instance
func NewLLMClient(mainEndpoint, summaryEndpoint string) (*LLMClient, error) {
	// Create main model client
	mainModel, err := openai.New(
		openai.WithBaseURL(mainEndpoint),
		openai.WithToken("dummy-token"), // Token might not be needed for local models
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create main model client: %w", err)
	}

	// Create summary model client (DeepSeek)
	summaryModel, err := openai.New(
		openai.WithBaseURL(summaryEndpoint),
		openai.WithToken("dummy-token"), // Token might not be needed for local models
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create summary model client: %w", err)
	}

	return &LLMClient{
		mainModel:    mainModel,
		summaryModel: summaryModel,
	}, nil
}

// GenerateCompletion generates completion using the main model
func (c *LLMClient) GenerateCompletion(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	response, err := c.mainModel.GenerateContent(ctx, []llms.MessageContent{
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
	_, err := c.mainModel.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}, append(options, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		return callback(string(chunk))
	}))...)

	return err
}

// SummarizeContent summarizes content using the summary model
func (c *LLMClient) SummarizeContent(ctx context.Context, content string) (string, error) {
	prompt := fmt.Sprintf(`Please summarize the following content while preserving the key information and context:

%s

Summary:`, content)

	response, err := c.summaryModel.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	})

	if err != nil {
		return "", fmt.Errorf("failed to summarize content: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	return response.Choices[0].Content, nil
}

// CountTokens estimates token count for a given text
func (c *LLMClient) CountTokens(text string) int {
	// Simple token estimation: roughly 4 characters per token
	// This is a rough approximation, for production use a proper tokenizer
	return len(text) / 4
}
