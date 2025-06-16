package utils

import (
	"fmt"
	"log"

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
		if msg.Role != types.RoleSystem {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// GetSystemMsg returns the first system message from messages
func GetSystemMsg(messages []types.Message) types.Message {
	for _, msg := range messages {
		if msg.Role == types.RoleSystem {
			return msg
		}
	}

	log.Printf("[GetSystemMsg] No system message found")
	return types.Message{Role: types.RoleSystem, Content: ""}
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
	latestUserMsg := GetRecentUserMsgsWithNum(messages, 1)
	if len(latestUserMsg) == 0 {
		return "", fmt.Errorf("no user message found")
	}
	return GetContentAsString(latestUserMsg[0].Content), nil
}

// GetOldUserMsgsWithNum returns messages between the first system message and the num-th last user message
func GetOldUserMsgsWithNum(messages []types.Message, num int) []types.Message {
	if num <= 0 {
		return messages
	}

	if num >= len(messages) {
		return []types.Message{}
	}

	// Assume system message is at position 0
	sysPos := 0
	if len(messages) == 0 || messages[0].Role != types.RoleSystem {
		// If not at 0, find the first system message
		for i := 0; i < len(messages); i++ {
			if messages[i].Role == types.RoleSystem {
				sysPos = i
				break
			}
		}
	}

	// Find position of num-th last user message
	userCount := 0
	userPos := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.RoleUser {
			userCount++
			if userCount == num {
				userPos = i
				break
			}
		}
	}

	// If no user message found, return all messages after system
	if userPos == -1 {
		log.Printf("[GetOldUserMsgsWithNum] No user message found")
		if sysPos >= len(messages)-1 {
			return []types.Message{}
		}
		return messages[sysPos+1:]
	}

	// Return messages between system and user positions
	if sysPos >= userPos {
		return []types.Message{}
	}
	return messages[sysPos+1 : userPos]
}

// GetRecentUserMsgsWithNum gets messages starting from the num-th user message from the end
// Returns messages from the position of the num-th user message from the end
func GetRecentUserMsgsWithNum(messages []types.Message, num int) []types.Message {
	if num <= 0 {
		return messages
	}

	// Find the position of the num-th user message from the end
	userCount := 0
	position := -1

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.RoleUser {
			userCount++
			if userCount == num {
				position = i
				break
			}
		}
	}

	// If we didn't find enough user messages, return empty slice
	if position == -1 {
		log.Println("[GetRecentUserMsgsWithNum] No user message found")
		return []types.Message{}
	}

	// Return messages from the position onwards
	return messages[position:]
}
