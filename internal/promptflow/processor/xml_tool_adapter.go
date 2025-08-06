package processor

import (
	"fmt"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/functions"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

type XmlToolAdapter struct {
	BaseProcessor
}

func NewXmlToolAdapter() *XmlToolAdapter {
	return &XmlToolAdapter{}
}

func (x *XmlToolAdapter) Execute(promptMsg *PromptMsg) {
	const method = "XmlToolAdapter.Execute"
	logger.Info("Start adapt xml tool to prompts", zap.String("method", method))

	if promptMsg == nil {
		x.Err = fmt.Errorf("received prompt message is empty")
		logger.Error(x.Err.Error(), zap.String("method", method))
		return
	}

	systemContent, err := x.extractSystemContent(promptMsg.systemMsg)
	if err != nil {
		logger.Warn("Failed to extract system message content",
			zap.String("method", method),
			zap.Error(err))
		x.Err = fmt.Errorf("failed to extract system message content: %w", err)
		x.passToNext(promptMsg)
		return
	}

	// Process system content to insert tools
	updatedContent, err := x.insertToolsIntoSystemContent(systemContent)
	if err != nil {
		logger.Warn("Failed to insert tools into system content",
			zap.String("method", method),
			zap.Error(err))
		x.Err = fmt.Errorf("failed to insert tools into system content: %w", err)
		x.passToNext(promptMsg)
		return
	}

	// Update the system message with the modified content
	promptMsg.UpdateSystemMsg(updatedContent)

	x.Handled = true
	x.passToNext(promptMsg)
}

// insertToolsIntoSystemContent inserts tool descriptions under the "# Tools" section
func (x *XmlToolAdapter) insertToolsIntoSystemContent(content string) (string, error) {
	const toolsHeader = "# Tools"

	// Combine all tools into a single string
	var toolsContent strings.Builder
	for _, tool := range functions.AvailableTools {
		toolsContent.WriteString(tool)
		toolsContent.WriteString("\n\n")
	}

	// Find the tools section
	headerIndex := strings.Index(content, toolsHeader)
	if headerIndex == -1 {
		return content, fmt.Errorf("tools header not found in system content")
	}

	// Find the end of the tools header line
	lineEnd := strings.Index(content[headerIndex:], "\n")
	if lineEnd == -1 {
		lineEnd = len(content) - headerIndex
	}
	insertPos := headerIndex + lineEnd + 1

	// Insert the tools content after the tools header
	result := content[:insertPos] + "\n" + toolsContent.String() + content[insertPos:]
	return result, nil
}
