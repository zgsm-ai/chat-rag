package processor

import (
	"fmt"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

type UserMsgFilter struct {
	BaseProcessor

	enableEnvDetailsFilter bool
}

func NewUserMsgFilter(enableEnvDetailsFilter bool) *UserMsgFilter {
	return &UserMsgFilter{
		enableEnvDetailsFilter: enableEnvDetailsFilter,
	}
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
	if u.enableEnvDetailsFilter {
		u.filterEnvironmentDetails(promptMsg)
	}

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

// removeToolExecutionPatterns removes strings that executing tool
// Temporarily hardcoded
func (u *UserMsgFilter) removeToolExecutionPatterns(content string) string {
	startPattern := types.StrFilterToolSearchStart
	endPattern := types.StrFilterToolSearchEnd + "....."

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

	// Remove the specific string
	thinkPattern := types.StrFilterToolAnalyzing + "..."
	result = strings.ReplaceAll(result, thinkPattern, "")
	if result != content {
		logger.Info("removed thinking... content", zap.String("method", "removeToolExecutionPatterns"))
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

// filterEnvironmentDetails removes environment details content from user messages
// Keeps the first occurrence of <environment_details> and removes subsequent ones
func (u *UserMsgFilter) filterEnvironmentDetails(promptMsg *PromptMsg) {
	const method = "UserMsgFilter.filterEnvironmentDetails"
	const environment_details = "<environment_details>"

	removedCount := 0
	environmentDetailsCount := 0

	for i := range promptMsg.olderUserMsgList {
		msg := &promptMsg.olderUserMsgList[i]

		// Only process user messages
		if msg.Role != types.RoleUser {
			continue
		}

		// Check if msg.Content is a list
		contentList, ok := msg.Content.([]interface{})
		if !ok {
			continue
		}

		// Filter out environment details from content list in place
		for j := 0; j < len(contentList); {
			item := contentList[j]

			// Try to extract text from the content item
			textStr := utils.ExtractTextFromContent(item)
			if textStr == "" {
				j++
				continue
			}

			// Check if this is environment details
			if strings.HasPrefix(textStr, environment_details) {
				environmentDetailsCount++
				if environmentDetailsCount > 1 {
					// Remove this element by slicing it out (skip the first one)
					contentList = append(contentList[:j], contentList[j+1:]...)
					removedCount++
					// Don't increment j since we removed the current element
					continue
				}
			}

			// Move to next element
			j++
		}

		// Update msg.Content with the modified slice
		msg.Content = contentList

	}

	logger.Info("[environment details] filtering completed",
		zap.Int("removed_count", removedCount),
		zap.String("method", method))
}
