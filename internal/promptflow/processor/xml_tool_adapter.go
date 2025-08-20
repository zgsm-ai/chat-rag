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
	var capabilitiesContent strings.Builder

	for _, toolName := range x.toolExecutor.GetAllTools() {
		ready, err := x.toolExecutor.CheckToolReady(x.ctx, toolName)
		if !ready {
			logger.WarnC(x.ctx, "Tool is not ready, skip adapt", zap.String("tool", toolName),
				zap.String("method", method), zap.Error(err))
			continue
		}

		desc, err := x.toolExecutor.GetToolDescription(toolName)
		if err != nil {
			logger.Error("Failed to get tool description", zap.Error(err))
			continue
		}

		toolsContent.WriteString(desc)
		toolsContent.WriteString("\n\n")

		// Get tool capability
		capability, err := x.toolExecutor.GetToolCapability(toolName)
		if err != nil {
			logger.Error("Failed to get tool capability", zap.Error(err))
			continue
		}
		capabilitiesContent.WriteString(capability)
		logger.InfoC(x.ctx, "Tool adapted in system prompt", zap.String("name", toolName))
	}

	// Insert the tools content after the tools header
	result, err := insertContentAfterMarker(content, "# Tools", toolsContent.String())
	if err != nil {
		return content, fmt.Errorf("failed to insert tools content: %w", err)
	}

	// Insert tool capabilities after CAPABILITIES section
	result, err = insertContentAfterMarker(result, "\n\n====\n\nCAPABILITIES\n\n", capabilitiesContent.String())
	if err != nil {
		return result, fmt.Errorf("failed to insert capabilities content: %w", err)
	}

	// Insert tools rules at the end
	toolsRules := x.toolExecutor.GetToolsRules()
	result = result + "\n" + toolsRules

	return result, nil
}

// insertContentAfterMarker inserts content after a specific marker in the text
func insertContentAfterMarker(content, marker, newContent string) (string, error) {
	markerIndex := strings.Index(content, marker)
	if markerIndex == -1 {
		return content, fmt.Errorf("marker not found in content")
	}

	// For headers like "# Tools", find the end of the line
	if strings.HasPrefix(marker, "#") {
		lineEnd := strings.Index(content[markerIndex:], "\n")
		if lineEnd == -1 {
			lineEnd = len(content) - markerIndex
		}
		insertPos := markerIndex + lineEnd + 1
		return content[:insertPos] + "\n" + newContent + content[insertPos:], nil
	}

	// For other markers, insert immediately after the marker
	insertPos := markerIndex + len(marker)
	return content[:insertPos] + newContent + content[insertPos:], nil
}
