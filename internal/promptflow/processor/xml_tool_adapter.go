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

const CodeAnalysisRules = `
Code Analysis Execution Rules
Rule 1: Tool Priority Hierarchy
1. codebase_search (Mandatory first step)
2. code_definition_search (For specific implementations)
3. read_file (Only when necessary for detailed analysis)
4. search_files (For regex pattern matching)

Rule 2: Decision Flow for Code Analysis Tasks
Receive code analysis task →
Use codebase_search with natural language query →
Review search results →
IF specific definition or implementations needed → Use code_definition_search
ELSE IF detailed file content required → Use read_file
ELSE IF pattern matching needed → Use search_files
END IF

Rule 3: Efficiency Principles
Semantic First: Always prefer semantic understanding over literal reading
Definition Search First: Always prefer definition searching over file reading
Comprehensive Coverage: Use codebase_search to avoid missing related code
Token Optimization: Choose tools that minimize token consumption
`

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
	var hasCodebaseSearch bool
	var hasCodeDefinitionSearch bool
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
		}

		toolsContent.WriteString(desc)
		toolsContent.WriteString("\n\n")
		logger.InfoC(x.ctx, "Tool adapted in system prompt", zap.String("name", toolName))

		// Check if this is codebase_search tool
		if toolName == "codebase_search" {
			hasCodebaseSearch = true
		}

		// Check if this is code_definition_search tool
		if toolName == "code_definition_search" {
			hasCodeDefinitionSearch = true
		}
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

	// If codebase_search tool is present, add description before MODES section
	if hasCodebaseSearch {
		const modesSection = "\n\n====\n\nMODES"
		modesIndex := strings.Index(result, modesSection)
		if modesIndex != -1 {
			codebaseSearchDesc := `- You can use codebase_search to perform semantic-aware searches across your codebase, returning conceptually relevant code snippets based on meaning rather than exact text matches. This is particularly powerful for discovering related functionality, exploring unfamiliar code architecture, or locating implementations when you only understand the purpose but not the specific syntax. For optimal efficiency, always try codebase_search first as it delivers more focused results with lower token consumption. Reserve other tools for cases where you need literal pattern matching or precise line-by-line analysis of file contents. This balanced approach ensures you get the right search method for each scenario - semantic discovery through codebase_search when possible, falling back to exhaustive text search via other tools only when necessary.`
			result = result[:modesIndex] + "\n" + codebaseSearchDesc + result[modesIndex:]
		}
	}

	// If code_definition_search tool is present, add description before MODES section
	if hasCodeDefinitionSearch {
		const modesSection = "\n\n====\n\nMODES"
		modesIndex := strings.Index(result, modesSection)
		if modesIndex != -1 {
			codeDefinitionSearchDesc := `- You can use code_definition_search to retrieve the full implementation of a symbol (function, class, method, interface, etc.) from the codebase by specifying its exact file path and line range. This is especially useful when you already know the location of a definition and need its complete code content, including precise position details for reference or modification. The tool provides accurate, context-free extraction of definitions, ensuring you get exactly the implementation you need without unnecessary surrounding code. For optimal efficiency, always use code_definition_search first when you have the file path and line numbers—it delivers fast, precise results with minimal overhead. If you need to search for related definitions without knowing their exact locations, consider using codebase_search (for semantic matches) or search_files (for regex-based scanning) as fallback options.`
			result = result[:modesIndex] + "\n" + codeDefinitionSearchDesc + result[modesIndex:]
		}
	}

	if hasCodeDefinitionSearch || hasCodebaseSearch {
		codeDefinitionSearchDesc := `- You can use codebase_search and code_definition_search individually or in combination: codebase_search helps you find broad code-related information based on natural language queries, while code_definition_search is perfect for pinpointing specific code definitions and their detailed contents. Only if the results from these two tools are insufficient should you resort to secondary tools for more granular searches.`
		result = result + "\n\nTOOLS USE FOLLOW RULES\n" + codeDefinitionSearchDesc + "\n" + CodeAnalysisRules
	}

	if hasCodeDefinitionSearch {
		codeDesc := `- You can use code_definition_search tool in various development tasks. In code generation, it helps quickly find relevant definitions of existing data structures, functions, or methods when creating new code. During code reviews, it enables you to easily locate and review definitions that may be in different files, ensuring proper implementation across the codebase. In unit testing, it allows you to identify necessary definitions from other files, making it easier to set up comprehensive tests. Additionally, for code understanding, this tool provides detailed access to code definitions, helping developers comprehend how different parts of the code interact and function, particularly in complex or unfamiliar systems.\n- If you want to know how a specific method is defined and implemented, simply use the code_definition_search tool, as it provides a fast and accurate way to retrieve the complete definition, making it superior to other ways.`
		result = result + "\n" + codeDesc
	}

	return result, nil
}
