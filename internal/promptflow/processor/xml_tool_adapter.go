package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/functions"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

type XmlToolAdapter struct {
	BaseProcessor

	ctx          context.Context
	toolExecutor functions.ToolExecutor
}

func NewXmlToolAdapter(ctx context.Context, toolExecutor functions.ToolExecutor) *XmlToolAdapter {
	return &XmlToolAdapter{
		ctx:          ctx,
		toolExecutor: toolExecutor,
	}
}

func (x *XmlToolAdapter) Execute(promptMsg *PromptMsg) {
	const method = "XmlToolAdapter.Execute"
	logger.InfoC(x.ctx, "Start adapt xml tool to prompts", zap.String("method", method))

	if promptMsg == nil {
		x.Err = fmt.Errorf("received prompt message is empty")
		logger.Error(x.Err.Error(), zap.String("method", method))
		return
	}

	systemContent, err := x.extractSystemContent(promptMsg.systemMsg)
	if err != nil {
		logger.WarnC(x.ctx, "Failed to extract system message content",
			zap.String("method", method),
			zap.Error(err))
		x.Err = fmt.Errorf("failed to extract system message content: %w", err)
		x.passToNext(promptMsg)
		return
	}

	// Process system content to insert tools
	updatedContent, err := x.insertToolsIntoSystemContent(systemContent)
	if err != nil {
		logger.WarnC(x.ctx, "Failed to insert tools into system content",
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
	const method = "XmlToolAdapter.insertToolsIntoSystemContent"
	if len(x.toolExecutor.GetAllTools()) == 0 {
		logger.InfoC(x.ctx, "No tools available", zap.String("method", method))
	}

	// Combine all tools into a single string
	var toolsContent strings.Builder
	for _, toolName := range x.toolExecutor.GetAllTools() {
		ready, err := x.toolExecutor.CheckToolReady(x.ctx, toolName)
		if !ready {
			logger.WarnC(x.ctx, "Tool is not ready, skip adapt", zap.String("method", method),
				zap.String("tool", toolName), zap.Error(err))
			continue
		}

		desc, err := x.toolExecutor.GetToolDescription(toolName)
		if err != nil {
			logger.Error("Failed to get tool description", zap.Error(err))
		}

		toolsContent.WriteString(desc)
		toolsContent.WriteString("\n\n")
		logger.InfoC(x.ctx, "Tool adapted in system prompt", zap.String("name", toolName))
	}

	// Find the tools section
	const toolsHeader = "# Tools"
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
