package utils

import (
	"encoding/json"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

// TokenCounter provides token counting functionality
type TokenCounter struct {
	encoder *tiktoken.Tiktoken
}

// NewTokenCounter creates a new token counter instance
func NewTokenCounter() (*TokenCounter, error) {
	// Use cl100k_base encoding (used by GPT-3.5 and GPT-4)
	encoder, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, err
	}

	return &TokenCounter{
		encoder: encoder,
	}, nil
}

// CountTokens counts tokens in a text string
func (tc *TokenCounter) CountTokens(text string) int {
	if tc.encoder == nil {
		// Fallback to simple estimation if encoder is not available
		return len(strings.Fields(text)) * 4 / 3 // Rough approximation
	}

	tokens := tc.encoder.Encode(text, nil, nil)
	return len(tokens)
}

// CountMessagesTokens counts tokens in a slice of messages
func (tc *TokenCounter) CountMessagesTokens(messages []map[string]interface{}) int {
	totalTokens := 0

	for _, message := range messages {
		// Count tokens for role
		if role, ok := message["role"].(string); ok {
			totalTokens += tc.CountTokens(role)
		}

		// Count tokens for content
		if content, ok := message["content"]; ok {
			totalTokens += tc.CountTokens(GetContentAsString(content))
		}

		// Add overhead tokens per message (approximately 3 tokens per message)
		totalTokens += 3
	}

	// Add overhead tokens for the conversation (approximately 3 tokens)
	totalTokens += 3
	return totalTokens
}

// CountJSONTokens counts tokens in a JSON object
func (tc *TokenCounter) CountJSONTokens(data interface{}) int {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return 0
	}

	return tc.CountTokens(string(jsonBytes))
}

// EstimateTokens provides a simple token estimation without tiktoken
func EstimateTokens(text string) int {
	// Simple estimation: roughly 4 characters per token
	return len(text) / 4
}
