package client

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// LLMClient handles communication with language models
type LLMClient struct {
	model     llms.Model
	modelName string
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

	return &LLMClient{
		model:     model,
		modelName: modelName,
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

// SummarizeContent summarizes content using the summary model
func (c *LLMClient) SummarizeContent(ctx context.Context, content string) (string, error) {
	prompt := fmt.Sprintf(`Please summarize the following content while preserving the key information and context: %s Summary:`, content)

	response, err := c.model.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	})

	if err != nil {
		logx.Errorw("failed to summarize content", logx.Field("error", err))
		return "", fmt.Errorf("failed to summarize content: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	return response.Choices[0].Content, nil
}

// SummarizeContentWithMessages summarizes content using a structured message format
func (c *LLMClient) SummarizeContentWithMessages(ctx context.Context, messages []types.Message) (string, error) {
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

		langchainMessages = append(langchainMessages, llms.TextParts(msgType, msg.Content))
	}

	// Generate summary using structured messages
	response, err := c.model.GenerateContent(ctx, langchainMessages)

	if err != nil {
		logx.Errorw("failed to summarize content with messages", logx.Field("error", err))
		return "", fmt.Errorf("failed to summarize content with messages: %w", err)
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
