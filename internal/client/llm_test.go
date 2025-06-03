package client

import (
	"context"
	"fmt"
	"testing"

	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

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
	c := utils.MustLoadConfig("../../etc/chat-api.yaml")

	client, err := NewLLMClient(c.LLMEndpoint, c.SummaryModel)
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
			summary, err := client.GenerateContent(ctx, "", tc.messages)

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
