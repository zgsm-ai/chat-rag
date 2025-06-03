package strategy

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

func TestSummaryProcessor_GenerateSummary(t *testing.T) {
	// load config
	c := utils.MustLoadConfig("../../etc/chat-api.yaml")

	// create llm client
	llmClient, err := client.NewLLMClient(
		c.LLMEndpoint,  // summary endpoint
		c.SummaryModel, // summary model
	)
	assert.NoError(t, err)

	// create summary processor
	processor := NewSummaryProcessor(llmClient)

	// Test cases
	tests := []struct {
		name              string
		semanticContext   string
		messages          []types.Message
		latestUserMessage string
		wantErr           bool
	}{
		{
			name:            "basic conversation summary",
			semanticContext: "Test context",
			messages: []types.Message{
				{
					Role:    "user",
					Content: "Hello, can you help me with something?",
				},
				{
					Role:    "assistant",
					Content: "Of course! What can I help you with?",
				},
			},
			latestUserMessage: "I need help with coding",
			wantErr:           false,
		},
		{
			name:            "with system message",
			semanticContext: "Test context",
			messages: []types.Message{
				{
					Role:    "system",
					Content: "You are a helpful assistant",
				},
				{
					Role:    "user",
					Content: "Hello",
				},
			},
			latestUserMessage: "Test message",
			wantErr:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := processor.GenerateUserPromptSummary(context.Background(), tt.semanticContext, tt.messages)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, summary)
			} else {
				fmt.Println("==> summary:", summary)
				assert.NoError(t, err)
				assert.NotEmpty(t, summary)
				// Verify summary contains key components
				assert.Contains(t, summary, "Previous Conversation")
				assert.Contains(t, summary, "Current Work")
				assert.Contains(t, summary, "Key Technical Concepts")
			}
		})
	}
}

func TestSummaryProcessor_BuildSummaryMessages(t *testing.T) {
	processor := &SummaryProcessor{}

	tests := []struct {
		name     string
		messages []types.Message
		summary  string
		want     int // expected number of messages
	}{
		{
			name: "with system message",
			messages: []types.Message{
				{
					Role:    "system",
					Content: "You are a helpful assistant",
				},
			},
			summary: "Test summary",
			want:    3, // system + summary + latest message
		},
		{
			name: "without system message",
			messages: []types.Message{
				{
					Role:    "user",
					Content: "Hello",
				},
			},
			summary: "Test summary",
			want:    2, // summary + latest message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.BuildUserSummaryMessages(context.Background(), tt.messages, tt.summary)
			assert.Equal(t, tt.want, len(result))

			// Verify message structure
			if len(tt.messages) > 0 && tt.messages[0].Role == "system" {
				assert.Equal(t, "system", result[0].Role)
			}

			// Verify summary message
			summaryMsg := result[len(result)-2]
			assert.Equal(t, "assistant", summaryMsg.Role)
			assert.Contains(t, summaryMsg.Content, tt.summary)

			// Verify latest message
			latestMsg := result[len(result)-1]
			assert.Equal(t, "user", latestMsg.Role)
		})
	}
}
