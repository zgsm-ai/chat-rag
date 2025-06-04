package strategy

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

func TestDirectProcessor_ProcessPrompt(t *testing.T) {
	processor := NewDirectProcessor()

	testCases := []struct {
		name     string
		request  *types.ChatCompletionRequest
		expected *ProcessedPrompt
	}{
		{
			name: "Simple user message",
			request: &types.ChatCompletionRequest{
				Model: "gpt-3.5-turbo",
				Messages: []types.Message{
					{Role: "user", Content: "Hello, how are you?"},
				},
			},
			expected: &ProcessedPrompt{
				Messages: []types.Message{
					{Role: "user", Content: "Hello, how are you?"},
				},
				IsCompressed:     false,
				OriginalTokens:   model.TokenStats{},
				CompressedTokens: model.TokenStats{},
				CompressionRatio: 1.0,
				SemanticLatency:  0,
				SummaryLatency:   0,
			},
		},
		{
			name: "Multiple messages",
			request: &types.ChatCompletionRequest{
				Model: "gpt-3.5-turbo",
				Messages: []types.Message{
					{Role: "system", Content: "You are a helpful assistant."},
					{Role: "user", Content: "What is Go programming?"},
					{Role: "assistant", Content: "Go is a programming language."},
					{Role: "user", Content: "Tell me more about it."},
				},
			},
			expected: &ProcessedPrompt{
				Messages: []types.Message{
					{Role: "system", Content: "You are a helpful assistant."},
					{Role: "user", Content: "What is Go programming?"},
					{Role: "assistant", Content: "Go is a programming language."},
					{Role: "user", Content: "Tell me more about it."},
				},
				IsCompressed:     false,
				OriginalTokens:   model.TokenStats{},
				CompressedTokens: model.TokenStats{},
				CompressionRatio: 1.0,
				SemanticLatency:  0,
				SummaryLatency:   0,
			},
		},
		{
			name: "Empty messages",
			request: &types.ChatCompletionRequest{
				Model:    "gpt-3.5-turbo",
				Messages: []types.Message{},
			},
			expected: &ProcessedPrompt{
				Messages:         []types.Message{},
				IsCompressed:     false,
				OriginalTokens:   model.TokenStats{},
				CompressedTokens: model.TokenStats{},
				CompressionRatio: 1.0,
				SemanticLatency:  0,
				SummaryLatency:   0,
			},
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := processor.ProcessPrompt(ctx, tc.request)

			if err != nil {
				t.Errorf("ProcessPrompt returned unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("ProcessPrompt returned nil result")
				return
			}

			// Compare messages
			if len(result.Messages) != len(tc.expected.Messages) {
				t.Errorf("Expected %d messages, got %d", len(tc.expected.Messages), len(result.Messages))
				return
			}

			for i, msg := range result.Messages {
				if msg.Role != tc.expected.Messages[i].Role {
					t.Errorf("Message %d: expected role %s, got %s", i, tc.expected.Messages[i].Role, msg.Role)
				}
				if msg.Content != tc.expected.Messages[i].Content {
					t.Errorf("Message %d: expected content %s, got %s", i, tc.expected.Messages[i].Content, msg.Content)
				}
			}

			// Compare other fields
			if result.IsCompressed != tc.expected.IsCompressed {
				t.Errorf("Expected IsCompressed %v, got %v", tc.expected.IsCompressed, result.IsCompressed)
			}
			if result.CompressionRatio != tc.expected.CompressionRatio {
				t.Errorf("Expected CompressionRatio %f, got %f", tc.expected.CompressionRatio, result.CompressionRatio)
			}
		})
	}
}

func TestCompressionProcessor_ProcessPrompt_NoUserMessage(t *testing.T) {
	// This test focuses on the error case where no user message is found
	// We'll use nil clients since we expect an early return with error
	processor := NewCompressionProcessor(nil, nil, 5)

	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []types.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "assistant", Content: "I'm ready to help."},
		},
	}

	result, err := processor.ProcessPrompt(ctx, req)

	if err == nil {
		t.Error("Expected error when no user message found, but got none")
		return
	}

	if result != nil {
		t.Error("Expected nil result when error occurs")
	}

	expectedError := "no user message found for semantic search"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got: %v", expectedError, err)
	}
}

func TestCompressionProcessor_ProcessPrompt_FindsLatestUserMessage(t *testing.T) {
	// This test verifies that the processor finds the latest user message
	// We'll test the logic by checking that it doesn't return the "no user message" error
	processor := NewCompressionProcessor(nil, nil, 5)

	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model:       "gpt-3.5-turbo",
		ClientId:    "test-client",
		ProjectPath: "/test/path",
		Messages: []types.Message{
			{Role: "user", Content: "First question about Python"},
			{Role: "assistant", Content: "Here's info about Python"},
			{Role: "user", Content: "Now tell me about Go"},
		},
	}

	// Even with nil clients, it should not return the "no user message" error
	// It will fail later in the process, but that's expected
	result, err := processor.ProcessPrompt(ctx, req)

	// We expect some error (due to nil clients), but not the "no user message" error
	if err != nil && strings.Contains(err.Error(), "no user message found") {
		t.Error("Should have found user message, but got 'no user message found' error")
	}

	// If no error, result should not be nil
	if err == nil && result == nil {
		t.Error("If no error, result should not be nil")
	}
}

func TestPromptProcessorFactory_CreateProcessor(t *testing.T) {
	// For factory testing, we can use nil clients since we're just testing the factory logic
	factory := NewPromptProcessorFactory(nil, nil, 10)

	testCases := []struct {
		name             string
		needsCompression bool
		expectedType     string
	}{
		{
			name:             "Create direct processor",
			needsCompression: false,
			expectedType:     "DirectProcessor",
		},
		{
			name:             "Create compression processor",
			needsCompression: true,
			expectedType:     "CompressionProcessor",
		},
	}

	header := make(http.Header)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			processor := factory.CreateProcessor(tc.needsCompression, &header)

			if processor == nil {
				t.Error("CreateProcessor returned nil")
				return
			}

			// Test the processor type by checking behavior with a simple request
			ctx := context.Background()
			req := &types.ChatCompletionRequest{
				Model: "gpt-3.5-turbo",
				Messages: []types.Message{
					{Role: "user", Content: "test"},
				},
			}

			result, err := processor.ProcessPrompt(ctx, req)

			if !tc.needsCompression {
				// Direct processor should work fine
				if err != nil {
					t.Errorf("Direct processor should not fail: %v", err)
					return
				}
				if result == nil {
					t.Error("Direct processor should return result")
					return
				}
				if result.IsCompressed {
					t.Error("Direct processor should not compress")
				}
			} else {
				// Compression processor will fail with nil clients, but we can check the error type
				if err == nil {
					t.Error("Compression processor with nil clients should fail")
					return
				}
				// Should fail due to missing clients, not due to missing user message
				if strings.Contains(err.Error(), "no user message found") {
					t.Error("Should not fail due to missing user message")
				}
			}
		})
	}
}

func TestNewDirectProcessor(t *testing.T) {
	processor := NewDirectProcessor()
	if processor == nil {
		t.Error("NewDirectProcessor returned nil")
	}
}

func TestNewCompressionProcessor(t *testing.T) {
	processor := NewCompressionProcessor(nil, nil, 5)
	if processor == nil {
		t.Error("NewCompressionProcessor returned nil")
	}
}

func TestNewPromptProcessorFactory(t *testing.T) {
	factory := NewPromptProcessorFactory(nil, nil, 10)
	if factory == nil {
		t.Error("NewPromptProcessorFactory returned nil")
	}
}

func TestProcessedPrompt_Structure(t *testing.T) {
	// Test the ProcessedPrompt structure
	prompt := &ProcessedPrompt{
		Messages: []types.Message{
			{Role: "user", Content: "test"},
		},
		IsCompressed:     true,
		OriginalTokens:   model.TokenStats{SystemTokens: 20, UserTokens: 80, All: 100},
		CompressedTokens: model.TokenStats{SystemTokens: 10, UserTokens: 40, All: 50},
		CompressionRatio: 0.5,
		SemanticLatency:  100,
		SummaryLatency:   200,
	}

	if len(prompt.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(prompt.Messages))
	}

	if !prompt.IsCompressed {
		t.Error("Expected IsCompressed to be true")
	}

	if prompt.OriginalTokens.All != 100 {
		t.Errorf("Expected OriginalTokens.All 100, got %d", prompt.OriginalTokens.All)
	}

	if prompt.CompressedTokens.All != 50 {
		t.Errorf("Expected CompressedTokens.All 50, got %d", prompt.CompressedTokens.All)
	}

	if prompt.CompressionRatio != 0.5 {
		t.Errorf("Expected CompressionRatio 0.5, got %f", prompt.CompressionRatio)
	}

	if prompt.SemanticLatency != 100 {
		t.Errorf("Expected SemanticLatency 100, got %d", prompt.SemanticLatency)
	}

	if prompt.SummaryLatency != 200 {
		t.Errorf("Expected SummaryLatency 200, got %d", prompt.SummaryLatency)
	}
}

func TestDirectProcessor_ProcessPrompt_PreservesOriginalMessages(t *testing.T) {
	processor := NewDirectProcessor()
	ctx := context.Background()

	originalMessages := []types.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "What is Go programming?"},
		{Role: "assistant", Content: "Go is a programming language developed by Google."},
		{Role: "user", Content: "Can you show me an example?"},
	}

	req := &types.ChatCompletionRequest{
		Model:    "gpt-3.5-turbo",
		Messages: originalMessages,
	}

	result, err := processor.ProcessPrompt(ctx, req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if result == nil {
		t.Error("Result should not be nil")
		return
	}

	// Check that all original messages are preserved
	if len(result.Messages) != len(originalMessages) {
		t.Errorf("Expected %d messages, got %d", len(originalMessages), len(result.Messages))
		return
	}

	for i, msg := range result.Messages {
		if msg.Role != originalMessages[i].Role {
			t.Errorf("Message %d: expected role %s, got %s", i, originalMessages[i].Role, msg.Role)
		}
		if msg.Content != originalMessages[i].Content {
			t.Errorf("Message %d: expected content %s, got %s", i, originalMessages[i].Content, msg.Content)
		}
	}

	// Verify no compression occurred
	if result.IsCompressed {
		t.Error("DirectProcessor should not compress messages")
	}

	if result.CompressionRatio != 1.0 {
		t.Errorf("Expected compression ratio 1.0, got %f", result.CompressionRatio)
	}
}

func TestCompressionProcessor_ProcessPrompt_WithRealLLMClient(t *testing.T) {
	// load config
	c := utils.MustLoadConfig("../../etc/chat-api.yaml")

	// create llm client
	llmClient, err := client.NewLLMClient(
		c.LLMEndpoint,  // summary endpoint
		c.SummaryModel, // summary model
	)
	assert.NoError(t, err)

	// create semantic client (need real client to avoid nil pointer)
	semanticClient := client.NewSemanticClient(c.SemanticApiEndpoint)
	processor := NewCompressionProcessor(semanticClient, llmClient, 5)

	// Verify processor was created successfully with real LLM client
	if processor == nil {
		t.Error("NewCompressionProcessor should not return nil with valid LLM client")
		return
	}

	// Test that the processor can be created without panicking
	// This verifies that llmClient parameter is not nil and properly initialized
	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model:       "gpt-3.5-turbo",
		ClientId:    "test-client",
		ProjectPath: "/test/path",
		Messages: []types.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Help me understand Go programming"},
		},
	}

	// This test verifies that the processor can be created with a real LLM client
	// The actual processing might fail due to external service unavailability, but
	// it should not fail due to nil LLM client issues
	result, err := processor.ProcessPrompt(ctx, req)

	if err != nil {
		t.Logf("ProcessPrompt failed (this might be expected if services are unavailable): %v", err)
		// Should not be a nil pointer error related to LLM client
		if strings.Contains(err.Error(), "nil pointer") {
			t.Errorf("Should not have nil pointer error with real LLM client: %v", err)
		}
	} else {
		// If successful, verify the result
		if result == nil {
			t.Error("Result should not be nil when processing succeeds")
		} else {
			t.Logf("ProcessPrompt succeeded with %d messages, compressed: %v",
				len(result.Messages), result.IsCompressed)
		}
	}
}

func TestCompressionProcessor_ProcessPrompt_WithRealClients(t *testing.T) {
	// load config
	c := utils.MustLoadConfig("../../etc/chat-api.yaml")

	// create llm client
	llmClient, err := client.NewLLMClient(
		c.LLMEndpoint,  // summary endpoint
		c.SummaryModel, // summary model
	)
	assert.NoError(t, err)

	// create semantic client
	semanticClient := client.NewSemanticClient(c.SemanticApiEndpoint)

	// create compression processor with real clients
	processor := NewCompressionProcessor(semanticClient, llmClient, c.TopK)

	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model:       c.SummaryModel,
		ClientId:    "test-client",
		ProjectPath: "/test/path",
		Messages: []types.Message{
			{Role: "system", Content: "You are a helpful assistant for Go programming."},
			{Role: "user", Content: "How do I create a simple HTTP server in Go?"},
			{Role: "user", Content: "How do I create a simple HTTP server in python?"},
			{Role: "user", Content: "How do I create a simple HTTP server in C++?"},
		},
	}

	// This test verifies that the processor works with real clients
	result, err := processor.ProcessPrompt(ctx, req)
	t.Logf("==> ProcessPrompt result: %+v", result)

	// Note: This test might fail if the external services are not available
	// but it should not fail due to nil client issues
	if err != nil {
		t.Logf("ProcessPrompt failed (this might be expected if services are unavailable): %v", err)
		// Check that it's not a nil pointer error
		if strings.Contains(err.Error(), "nil pointer") {
			t.Errorf("Should not have nil pointer error with real clients: %v", err)
		}
	} else {
		// If successful, verify the result
		if result == nil {
			t.Error("Result should not be nil when processing succeeds")
		} else {
			t.Logf("ProcessPrompt succeeded with %d messages, compressed: %v",
				len(result.Messages), result.IsCompressed)
		}
	}

	// Verify processor was created successfully
	if processor == nil {
		t.Error("NewCompressionProcessor should not return nil with valid clients")
	}
}
