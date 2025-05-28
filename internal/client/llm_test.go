package client

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zgsm-ai/chat-rag/internal/types"
)

const testLLMEndpoint = "http://localhost:8080/v1"
const testModel = "gpt-3.5-turbo"

func TestNewLLMClient(t *testing.T) {
	// Test successful client creation
	client, err := NewLLMClient(testLLMEndpoint, testModel)
	if err != nil {
		t.Fatalf("NewLLMClient returned unexpected error: %v", err)
	}

	if client == nil {
		t.Fatal("NewLLMClient returned nil client")
	}

	if client.modelName != "gpt-3.5-turbo" {
		t.Errorf("Expected model name 'gpt-3.5-turbo', got '%s'", client.modelName)
	}

	// Test with empty endpoint URL
	_, err = NewLLMClient("", "gpt-3.5-turbo")
	if err == nil {
		t.Error("Expected error with empty endpoint URL, but got nil")
	} else if !strings.Contains(err.Error(), "endpoint cannot be empty") {
		t.Errorf("Expected error message to contain 'endpoint cannot be empty', got: %v", err)
	}
}

func TestLLMClient_CountTokens(t *testing.T) {
	// Create client
	client, err := NewLLMClient(testLLMEndpoint, testModel)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	testCases := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"Hello", 1},
		{"Hello, world!", 3},
		{"This is a longer text that should be around 10 tokens or so.", 15},
	}

	for _, tc := range testCases {
		tokens := client.CountTokens(tc.input)
		// Since this is an approximation, we'll check if it's in a reasonable range
		// The actual implementation uses len(text) / 4
		expected := len(tc.input) / 4
		if tokens != expected {
			t.Errorf("Expected around %d tokens for '%s', got %d", expected, tc.input, tokens)
		}
	}
}

func TestLLMClient_ChatLLMWithMessages_FormatCheck(t *testing.T) {
	// Simple message format validation without creating an actual client
	messages := []struct {
		Role    string
		Content string
	}{
		{"system", "You are a helpful assistant that summarizes content."},
		{"user", "This is test content that needs to be summarized"},
	}

	// Verify messages contain expected content
	for _, msg := range messages {
		if msg.Content == "" {
			t.Errorf("Message content should not be empty")
		}
		if msg.Role == "" {
			t.Errorf("Message role should not be empty")
		}
	}
}

func TestLLMClient_ChatLLMWithMessages(t *testing.T) {
	// Create client for actual API testing
	client, err := NewLLMClient(testLLMEndpoint, testModel)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test cases
	testCases := []struct {
		name        string
		messages    []types.Message
		expectError bool
	}{
		{
			name:        "Empty messages",
			messages:    []types.Message{},
			expectError: true, // Empty messages should return an error
		},
		{
			name: "Single user message",
			messages: []types.Message{
				{Role: "user", Content: "This is a short text to summarize."},
			},
			expectError: false,
		},
		{
			name: "System and user messages",
			messages: []types.Message{
				{Role: "system", Content: "You are a helpful assistant that summarizes content."},
				{Role: "user", Content: "This is a longer text that contains multiple sentences. It discusses various topics and should be summarized properly. The summary should retain the key information while being concise."},
			},
			expectError: false,
		},
		{
			name: "Conversation with assistant",
			messages: []types.Message{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "Please summarize this content."},
				{Role: "assistant", Content: "I'll help you summarize the content."},
				{Role: "user", Content: "Here is the content: This is important information that needs to be condensed."},
			},
			expectError: false,
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			summary, err := client.ChatLLMWithMessages(ctx, tc.messages)

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if err == nil {
				// Check that summary is not empty when we successfully call the API
				if summary == "" {
					t.Error("Received empty summary")
				}
			}
			fmt.Println("messages:", tc.messages)
			fmt.Println("summary:", summary)
		})
	}
}

func TestLLMClient_Integration(t *testing.T) {
	// Skip this test as it requires external services
	t.Skip("Skipping integration test")

	// Note: This test is skipped. The following code is just an example.
	// If you want to run this test, remove t.Skip() and add the following import:
	// import "context"
}
