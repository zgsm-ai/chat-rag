package processor

import (
	"fmt"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/functions"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"go.uber.org/zap"
)

const (
	// ToolInstructionReminder this content will affect function call response.
	ToolInstructionReminder      = "# Reminder: Instructions for Tool Use\n\nTool uses are formatted using XML-style tags. The tool name is enclosed in opening and closing tags, and each parameter is similarly enclosed within its own set of tags. Here's the structure:\n\n<tool_name>\n<parameter1_name>value1</parameter1_name>\n<parameter2_name>value2</parameter2_name>\n...\n</tool_name>\n\nFor example:\n\n<attempt_completion>\n<result>\nI have completed the task...\n</result>\n</attempt_completion>\n"
	ToolInstructionReminderInErr = "# Reminder: Instructions for Tool Use\n\nTool uses are formatted using XML-style tags. The tool name itself becomes the XML tag name. Each parameter is enclosed within its own set of tags. Here's the structure:\n\n<actual_tool_name>\n<parameter1_name>value1</parameter1_name>\n<parameter2_name>value2</parameter2_name>\n...\n</actual_tool_name>\n\nFor example, to use the attempt_completion tool:\n\n<attempt_completion>\n<result>\nI have completed the task...\n</result>\n</attempt_completion>\n\nAlways use the actual tool name as the XML tag name for proper parsing and execution.\n"
)

type FunctionAdapter struct {
	BaseProcessor

	modelName         string
	funcCallingModels []string
	functionsManager  *functions.ToolManager
}

func NewFunctionAdapter(modelName string, funcCallingModels []string, functionsManager *functions.ToolManager) *FunctionAdapter {
	return &FunctionAdapter{
		modelName:         modelName,
		funcCallingModels: funcCallingModels,
		functionsManager:  functionsManager,
	}
}

// Execute implements the processor interface, handling function call adaptation logic
func (f *FunctionAdapter) Execute(promptMsg *PromptMsg) {
	const method = "FunctionAdapter.Execute"
	logger.Info("Start adapting function calls to prompts", zap.String("method", method))

	// 1. Parameter validation
	if promptMsg == nil {
		f.Err = fmt.Errorf("received prompt message is empty")
		logger.Error(f.Err.Error(), zap.String("method", method))
		return
	}

	// 2. Check if model supports function calling
	if !f.isModelSupported() {
		logger.Info("Model does not support function calls, skipping adaptation",
			zap.String("modelName", f.modelName),
			zap.String("method", method))
		f.passToNext(promptMsg)
		return
	}

	// 3. Extract system message content
	systemContent, err := f.extractSystemContent(promptMsg.systemMsg)
	if err != nil {
		logger.Warn("Failed to extract system message content",
			zap.String("method", method),
			zap.Error(err))
		f.Err = fmt.Errorf("failed to extract system message content: %w", err)
		f.passToNext(promptMsg)
		return
	}

	// 4. Process tool-related logic
	f.processTools(promptMsg, systemContent)

	// 5. Filter messages by content
	filterStrings := []string{
		ToolInstructionReminder,
		ToolInstructionReminderInErr,
	}
	f.filterMessagesByContent(promptMsg, filterStrings)

	// 6. Clean up tool descriptions in system message
	cleanedContent := f.cleanToolDescriptions(systemContent)
	promptMsg.SetSystemMsg(cleanedContent)

	f.Handled = true
	f.passToNext(promptMsg)
}

// extractSystemContent extracts content from system message
func (f *FunctionAdapter) extractSystemContent(systemMsg *types.Message) (string, error) {
	var content model.Content
	contents, err := content.ExtractMsgContent(systemMsg)
	if err != nil {
		return "", fmt.Errorf("failed to extract message content: %w", err)
	}

	if len(contents) != 1 {
		return "", fmt.Errorf("expected one system content, got %d", len(contents))
	}

	return contents[0].Text, nil
}

// processTools processes tool-related logic
func (f *FunctionAdapter) processTools(promptMsg *PromptMsg, systemContent string) {
	// 1. Get available tools
	availableTools := f.getAvailableTools(systemContent)
	logger.Info("Available tools", zap.Int("nums", len(availableTools)),
		zap.String("method", "FunctionAdapter.processTools"))

	// 2. Add tools to prompt message
	for _, toolName := range availableTools {
		if tool, exists := f.functionsManager.GetTool(toolName); exists {
			promptMsg.AddTool(tool)
		} else {
			logger.Warn("Tool not found",
				zap.String("toolName", toolName))
		}
	}
}

// getAvailableTools gets currently available tool list
func (f *FunctionAdapter) getAvailableTools(systemContent string) []string {
	var availableTools []string

	// 1. Add client tools
	for _, tool := range f.functionsManager.GetClientTools() {
		if strings.Contains(systemContent, fmt.Sprintf("## %s", tool)) {
			availableTools = append(availableTools, tool)
		}
	}

	// 2. Add server tools
	availableTools = append(availableTools, f.functionsManager.GetServerTools()...)

	return availableTools
}

// cleanToolDescriptions cleans up tool descriptions in system message
func (f *FunctionAdapter) cleanToolDescriptions(content string) string {
	// 1. Remove tool usage formatting section
	if toolStart := strings.Index(content, "# Tool Use Formatting"); toolStart != -1 {
		if toolEnd := strings.Index(content, "# Tool Use Guidelines"); toolEnd != -1 {
			content = content[:toolStart] + content[toolEnd:]
		}
	}

	// 2. Remove specific lines
	return strings.ReplaceAll(content,
		"4. Formulate your tool use using the XML format specified for each tool.", "")
}

// isModelSupported checks if current model is in the list of function-calling supported models
// Supports wildcard matching, e.g. "qwen3*" matches models starting with "qwen3"
func (f *FunctionAdapter) isModelSupported() bool {
	for _, supportedModel := range f.funcCallingModels {
		if f.matchModel(f.modelName, supportedModel) {
			return true
		}
	}
	return false
}

// matchModel checks if model name matches pattern (supports wildcards)
func (f *FunctionAdapter) matchModel(modelName, pattern string) bool {
	// Simple comparison (no wildcard)
	if !strings.Contains(pattern, "*") {
		return strings.EqualFold(modelName, pattern)
	}

	// Prefix matching (e.g. "qwen3*")
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(strings.ToLower(modelName), strings.ToLower(prefix))
	}

	// Suffix matching (e.g. "*claude")
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(strings.ToLower(modelName), strings.ToLower(suffix))
	}

	return false
}

// filterMessagesByContent filters messages based on content strings
func (f *FunctionAdapter) filterMessagesByContent(promptMsg *PromptMsg, filterStrings []string) {
	if len(promptMsg.olderUserMsgList) == 0 || len(filterStrings) == 0 {
		return
	}

	modifiedCount := 0

	for i := range promptMsg.olderUserMsgList {
		msg := &promptMsg.olderUserMsgList[i] // 获取指针以便修改
		modified := false

		switch v := msg.Content.(type) {
		case string:
			original := v
			for _, filterStr := range filterStrings {
				v = strings.ReplaceAll(v, filterStr, "")
			}
			if v != original {
				msg.Content = v
				modified = true
			}

		case []interface{}:
			contentSlice := make([]model.Content, 0, len(v))
			for _, item := range v {
				if content, ok := item.(map[string]interface{}); ok {
					c := model.Content{
						Type: model.ContTypeText,
						Text: content["text"].(string),
					}
					originalText := c.Text

					for _, filterStr := range filterStrings {
						c.Text = strings.ReplaceAll(c.Text, filterStr, "")
					}

					if c.Text != originalText {
						modified = true
					}
					contentSlice = append(contentSlice, c)
				}
			}
			if modified {
				msg.Content = contentSlice
			}

		default:
			continue
		}

		if modified {
			modifiedCount++
		}
	}
	if modifiedCount > 0 {
		logger.Info("Filtered messages by content",
			zap.Int("Filtered count", modifiedCount))
	}
}
