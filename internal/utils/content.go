package utils

import (
	"fmt"

	"github.com/zgsm-ai/chat-rag/internal/types"
)

const (
	ContentTypeText     = "text"
	ContentTypeImageURL = "image_url"
)

// GetContentAsString converts content to string without parsing internal structure
func GetContentAsString(content interface{}) string {
	// Returns raw JSON content directly
	con, ok := content.(string)
	if ok {
		return con
	}
	contentList, ok := content.([]any)
	if ok {
		var contentStr string
		for _, contentItem := range contentList {
			contentMap, ok := contentItem.(map[string]any)
			if !ok {
				continue
			}
			if contentMap["type"] == ContentTypeText {
				if subStr, ok := contentMap["text"].(string); ok {
					contentStr += subStr
				}
			}
		}
		return contentStr
	}
	return ""
}

// GetUserMsgs filters out non-system messages
func GetUserMsgs(messages []types.Message) []types.Message {
	filtered := make([]types.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != "system" {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// TruncateContent truncates content to a specified length for logging
func TruncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "..."
}

// GetLatestUserMsg gets the newest user message content from message list
func GetLatestUserMsg(messages []types.Message) (string, error) {
	// Search backwards from last message to find user message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return GetContentAsString(messages[i].Content), nil
		}
	}
	return "", fmt.Errorf("no user message found")
}

// GetOldUserMsgs filters out old user messages
func GetOldUserMsgs(messages []types.Message) []types.Message {
	var filtered []types.Message
	for i := 0; i < len(messages); i++ {
		// Skip system messages and last user message
		if messages[i].Role == "system" ||
			(messages[i].Role == "user" && i >= len(messages)-1) {
			continue
		}
		filtered = append(filtered, messages[i])
	}
	return filtered
}
