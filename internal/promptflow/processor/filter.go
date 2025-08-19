package processor

import (
	"fmt"

	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"go.uber.org/zap"
)

type UserMsgFilter struct {
	BaseProcessor
}

func NewUserMsgFilter() *UserMsgFilter {
	return &UserMsgFilter{}
}

func (u *UserMsgFilter) Execute(promptMsg *PromptMsg) {
	const method = "UserMsgFilter.Execute"
	logger.Info("Start user message filter to prompts", zap.String("method", method))

	if promptMsg == nil {
		u.Err = fmt.Errorf("received prompt message is empty")
		logger.Error(u.Err.Error(), zap.String("method", method))
		return
	}

	// Skip processing if there are no older messages
	if len(promptMsg.olderUserMsgList) == 0 {
		logger.Debug("No older user messages to filter", zap.String("method", method))
		u.passToNext(promptMsg)
		return
	}

	originalCount := len(promptMsg.olderUserMsgList)
	u.filterDuplicateMessages(promptMsg)
	u.filterAssistantToolPatterns(promptMsg)
	removedCount := originalCount - len(promptMsg.olderUserMsgList)

	logger.Info("User message filter completed",
		zap.Int("removed duplicate content count", removedCount),
		zap.String("method", method))

	u.Handled = true
	u.passToNext(promptMsg)
}

// filterDuplicateMessages removes duplicate string content messages, keeping the last occurrence
func (u *UserMsgFilter) filterDuplicateMessages(promptMsg *PromptMsg) {
	seenContents := make(map[string]struct{})
	filteredMessages := make([]types.Message, 0, len(promptMsg.olderUserMsgList))

	// Iterate in reverse to keep the last occurrence of each duplicate
	for i := len(promptMsg.olderUserMsgList) - 1; i >= 0; i-- {
		msg := promptMsg.olderUserMsgList[i]

		content, ok := msg.Content.(string)
		if !ok {
			// Include non-string content messages as-is
			filteredMessages = append(filteredMessages, msg)
			continue
		}

		// Skip if we've already seen this content
		if _, exists := seenContents[content]; exists {
			continue
		}

		// Mark content as seen and add to filtered list
		seenContents[content] = struct{}{}
		filteredMessages = append(filteredMessages, msg)
	}

	// Reverse back to original order (now with duplicates removed)
	for i, j := 0, len(filteredMessages)-1; i < j; i, j = i+1, j-1 {
		filteredMessages[i], filteredMessages[j] = filteredMessages[j], filteredMessages[i]
	}

	promptMsg.olderUserMsgList = filteredMessages
}

// TODO this func will be removed when client apapted tool status dispply
// filterAssistantToolPatterns removes tool execution patterns from assistant messages
func (u *UserMsgFilter) filterAssistantToolPatterns(promptMsg *PromptMsg) {
	for i := range promptMsg.olderUserMsgList {
		msg := &promptMsg.olderUserMsgList[i]

		// Only process assistant messages
		if msg.Role != types.RoleAssistant {
			continue
		}

		content, ok := msg.Content.(string)
		if !ok {
			// Skip non-string content messages
			continue
		}

		// Remove tool execution patterns
		msg.Content = u.removeToolExecutionPatterns(content)
	}
}

// removeToolExecutionPatterns removes strings that start with "#### 🔍 " and end with "工具执行中......."
func (u *UserMsgFilter) removeToolExecutionPatterns(content string) string {
	startPattern := "#### 🔍 "
	endPattern := "工具执行中......."

	result := content
	for {
		startIndex := u.indexOf(result, startPattern)
		if startIndex == -1 {
			break
		}

		endIndex := u.indexOf(result[startIndex:], endPattern)
		if endIndex == -1 {
			break
		}

		endIndex += startIndex // Adjust to original string index
		// Include the end pattern length
		endIndex += len(endPattern)

		// Remove the pattern
		result = result[:startIndex] + result[endIndex:]
		logger.Info("removed tool executing... content", zap.String("method", "removeToolExecutionPatterns"))
	}

	return result
}

// indexOf returns the index of the first occurrence of pattern in s
func (u *UserMsgFilter) indexOf(s, pattern string) int {
	for i := 0; i <= len(s)-len(pattern); i++ {
		if s[i:i+len(pattern)] == pattern {
			return i
		}
	}
	return -1
}
