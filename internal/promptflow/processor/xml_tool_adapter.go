package processor

import (
	"fmt"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

type XmlToolAdapter struct {
	BaseProcessor
}

func NewXmlToolAdapter() *XmlToolAdapter {
	return &XmlToolAdapter{}
}

const (
	CodebaseSearchTool = `## codebase_search
Description: Find files most relevant to the search query.\nThis is a semantic search tool, so the query should ask for something semantically matching what is needed.\nIf it makes sense to only search in a particular directory, please specify it in the path parameter.\nUnless there is a clear reason to use your own search query, please just reuse the user's exact query with their wording.\nTheir exact wording/phrasing can often be helpful for the semantic search query. Keeping the same exact question format can also be helpful.\nIMPORTANT: Queries MUST be in English. Translate non-English queries before searching.
Parameters:
- query: (required) The search query to find relevant code. You should reuse the user's exact query/most recent message with their wording unless there is a clear reason not to.
- path: (optional) The path to the directory to search in relative to the current working directory. This parameter should only be a directory path, file paths are not supported. Defaults to the current working directory.
Usage:
<codebase_search>
<query>Your natural language query here</query>
<path>Path to the directory to search in (optional)</path>
</codebase_search>

Example: Searching for functions related to user authentication
<codebase_search>
<query>User login and password hashing</query>
<path>/path/to/directory</path>
</codebase_search>

Importance: If use this tool, don't need to think, just give the tool directly
`
)

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

	// Define all available tools (can be expanded with more tools)
	tools := []string{
		CodebaseSearchTool,
		// Add other tools here if needed
	}

	// Combine all tools into a single string
	var toolsContent strings.Builder
	for _, tool := range tools {
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
