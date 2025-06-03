package utils

import "github.com/zgsm-ai/chat-rag/internal/types"

const (
	ContentTypeText     = "text"
	ContentTypeImageURL = "image_url"
)

// GetContentAsString 将内容转换为字符串，不解析内部结构
func GetContentAsString(content interface{}) string {
	// 直接返回原始JSON内容
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

// GetUserMessages 过滤出非system消息
func GetUserMessages(messages []types.Message) []types.Message {
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
