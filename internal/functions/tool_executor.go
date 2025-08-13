package functions

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

const (
	CodebaseSearchToolName = "codebase_search"
	CodebaseSearchToolDesc = `## codebase_search
Description: 
Find files most relevant to the search query.
This is a semantic search tool, so the query should ask for something semantically matching what is needed.
If it makes sense to only search in a particular directory, please specify it in the path parameter.
Unless there is a clear reason to use your own search query, please just reuse the user's exact query with their wording.
Their exact wording/phrasing can often be helpful for the semantic search query. 
Keeping the same exact question format can also be helpful.
IMPORTANT: Queries MUST be in English. Translate non-English queries before searching.

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
`

	RelationSearchToolName = "relation_search"
	RelationSearchToolDesc = `## relation_search
Description:
Find the definition of a symbol and retrieve its **reference relationships** in a tree structure.
This tool takes a code location (by file and range) and optionally a symbol name, and returns its definition and all references (across files) with their precise code locations.
Use this tool when the user wants to understand **how a function/class is used or called**, or **explore code dependencies** from a specific location.
You MUST specify the file path and the exact code range (line and column positions) to locate the target symbol.
You can optionally provide a symbol name if it's known (e.g., function name), and enable content fetching if needed.
The result is returned as a **tree structure**, where the root is the definition node and children are reference nodes (possibly nested).

Parameters:
- filePath: (required) Relative path to the file where the symbol is located (e.g., src/utils/math.go).
- startLine: (required) The line number where the symbol starts.
- startColumn: (required) The column number where the symbol starts.
- endLine: (required) The line number where the symbol ends.
- endColumn: (required) The column number where the symbol ends.
- symbolName: (optional) The name of the symbol (e.g., function name, class name). Use this only if you're confident about the symbol.
- includeContent: (optional) Set to 1 if you need the actual code content of the definition and references. Defaults to 0.
- maxLayer: (optional) Maximum number of reference levels to follow (default is 10). Use smaller values for simpler trees.

Usage:
<relation_search>
  <filePath>Relative path to the file containing the symbol</filePath>
  <startLine>Start line number of the symbol (1-based)</startLine>
  <startColumn>Start column number of the symbol (1-based)</startColumn>
  <endLine>End line number of the symbol (1-based)</endLine>
  <endColumn>End column number of the symbol (1-based)</endColumn>
  <symbolName>Symbol name (optional)</symbolName>
  <includeContent>1 to include code content, 0 or omit to skip (optional)</includeContent>
  <maxLayer>Maximum number of reference layers to return (optional, default 10)</maxLayer>
</relation_search>


Example: Exploring all references to the GetUserById function
<relation_search>
  <filePath>src/services/user_service.go</filePath>
  <startLine>12</startLine>
  <startColumn>5</startColumn>
  <endLine>14</endLine>
  <endColumn>1</endColumn>
  <symbolName>GetUserById</symbolName>
  <includeContent>1</includeContent>
  <maxLayer>3</maxLayer>
</relation_search>
`
)

type ToolExecutor interface {
	DetectTools(ctx context.Context, content string) (bool, string)

	// ExecuteTools executes tools and returns new messages
	ExecuteTools(ctx context.Context, toolName string, content string) (string, error)

	CheckToolReady(ctx context.Context, toolName string) (bool, error)

	GetToolDescription(toolName string) (string, error)

	GetAllTools() []string
}

// ToolFunc represents a tool with its execute and ready check functions
type ToolFunc struct {
	description string
	execute     func(context.Context, string) (string, error)
	readyCheck  func(context.Context) (bool, error)
}

type XmlToolExecutor struct {
	tools map[string]ToolFunc
}

// NewXmlToolExecutor creates a new XmlToolExecutor instance
func NewXmlToolExecutor(
	c config.SemanticSearchConfig,
	semanticClient client.SemanticInterface,
	relationClient client.RelationInterface,
) *XmlToolExecutor {
	return &XmlToolExecutor{
		tools: map[string]ToolFunc{
			CodebaseSearchToolName: createCodebaseSearchTool(c, semanticClient),
			// RelationSearchToolName: createRelationSearchTool(relationClient),
		},
	}
}

// createCodebaseSearchTool creates the codebase search tool function
func createCodebaseSearchTool(c config.SemanticSearchConfig, semanticClient client.SemanticInterface) ToolFunc {
	return ToolFunc{
		description: CodebaseSearchToolDesc,
		execute: func(ctx context.Context, param string) (string, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return "", err
			}

			query, err := extractXmlParam(param, "query")
			if err != nil {
				return "", fmt.Errorf("failed to extract query: %w", err)
			}

			result, err := semanticClient.Search(ctx, client.SemanticRequest{
				ClientId:      identity.ClientID,
				CodebasePath:  identity.ProjectPath,
				Query:         query,
				TopK:          c.TopK,
				Authorization: identity.AuthToken,
				Score:         c.ScoreThreshold,
			})
			if err != nil {
				return "", fmt.Errorf("semantic search failed: %w", err)
			}

			return utils.MarshalJSONWithoutEscapeHTML(result.Results)
		},
		readyCheck: func(ctx context.Context) (bool, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return false, err
			}

			return semanticClient.CheckReady(ctx, client.ReadyRequest{
				ClientId:     identity.ClientID,
				CodebasePath: identity.ProjectPath,
			})
		},
	}
}

// createRelationSearchTool creates the relation search tool function
func createRelationSearchTool(relationClient client.RelationInterface) ToolFunc {
	return ToolFunc{
		description: RelationSearchToolDesc,
		execute: func(ctx context.Context, param string) (string, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return "", err
			}

			req, err := buildRelationRequest(identity, param)
			if err != nil {
				return "", fmt.Errorf("failed to build request: %w", err)
			}

			result, err := relationClient.Search(ctx, req)
			if err != nil {
				return "", fmt.Errorf("relation search failed: %w", err)
			}

			return utils.MarshalJSONWithoutEscapeHTML(result)
		},
	}
}

// buildRelationRequest constructs a RelationRequest from XML parameters
func buildRelationRequest(identity *model.Identity, param string) (client.RelationRequest, error) {
	req := client.RelationRequest{
		ClientId:      identity.ClientID,
		CodebasePath:  identity.ProjectPath,
		Authorization: identity.AuthToken,
	}

	var err error
	if req.FilePath, err = extractXmlParam(param, "filePath"); err != nil {
		return req, fmt.Errorf("filePath: %w", err)
	}

	if req.StartLine, err = extractXmlIntParam(param, "startLine"); err != nil {
		return req, fmt.Errorf("startLine: %w", err)
	}

	if req.StartColumn, err = extractXmlIntParam(param, "startColumn"); err != nil {
		return req, fmt.Errorf("startColumn: %w", err)
	}

	if req.EndLine, err = extractXmlIntParam(param, "endLine"); err != nil {
		return req, fmt.Errorf("endLine: %w", err)
	}

	if req.EndColumn, err = extractXmlIntParam(param, "endColumn"); err != nil {
		return req, fmt.Errorf("endColumn: %w", err)
	}

	// Optional parameters
	if symbolName, err := extractXmlParam(param, "symbolName"); err == nil {
		req.SymbolName = symbolName
	}

	if includeContent, err := extractXmlIntParam(param, "includeContent"); err == nil {
		req.IncludeContent = includeContent
	}

	if maxLayer, err := extractXmlIntParam(param, "maxLayer"); err == nil {
		req.MaxLayer = maxLayer
	}

	return req, nil
}

// Helper functions

func getIdentityFromContext(ctx context.Context) (*model.Identity, error) {
	identity, exists := model.GetIdentityFromContext(ctx)
	if !exists {
		return nil, fmt.Errorf("identity not found in context")
	}
	return identity, nil
}

func extractXmlParam(content, paramName string) (string, error) {
	startTag := "<" + paramName + ">"
	endTag := "</" + paramName + ">"

	start := strings.Index(content, startTag)
	if start == -1 {
		return "", fmt.Errorf("start tag not found")
	}

	end := strings.Index(content, endTag)
	if end == -1 {
		return "", fmt.Errorf("end tag not found")
	}

	return content[start+len(startTag) : end], nil
}

func extractXmlIntParam(content, paramName string) (int, error) {
	param, err := extractXmlParam(content, paramName)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(param)
}

// Implement remaining ToolExecutor interface methods...

// DetectTools only detects if tool calls are included and extracts tool information
// Returns: whether tool is detected, tool name
func (x *XmlToolExecutor) DetectTools(ctx context.Context, content string) (bool, string) {
	for toolName := range x.tools {
		if strings.Contains(content, "<"+toolName+">") {
			return true, toolName
		}
	}
	return false, ""
}

// ExecuteTools executes the specified tool and constructs new messages
func (x *XmlToolExecutor) ExecuteTools(ctx context.Context, toolName string, content string) (string, error) {
	// Get tool function
	toolFunc, exists := x.tools[toolName]
	if !exists {
		return "", fmt.Errorf("tool %s not found", toolName)
	}

	param, err := extractXmlParam(content, toolName)
	if err != nil {
		return "", fmt.Errorf("failed to extract tool parameters: %w", err)
	}

	return toolFunc.execute(ctx, param)
}

// CheckApiReady checks if the tool is ready to use
func (x *XmlToolExecutor) CheckToolReady(ctx context.Context, toolName string) (bool, error) {
	toolFunc, exists := x.tools[toolName]
	if !exists {
		return false, fmt.Errorf("tool %s not found", toolName)
	}

	// tool does not require ready check
	if toolFunc.readyCheck == nil {
		return true, nil
	}

	return toolFunc.readyCheck(ctx)
}

// GetToolDescription returns the description of the specified tool
func (x *XmlToolExecutor) GetToolDescription(toolName string) (string, error) {
	toolFunc, exists := x.tools[toolName]
	if !exists {
		return "", fmt.Errorf("tool %s not found", toolName)
	}

	return toolFunc.description, nil
}

// GetAllTools returns the names of all registered tools
func (x *XmlToolExecutor) GetAllTools() []string {
	tools := make([]string, 0, len(x.tools))
	for name := range x.tools {
		tools = append(tools, name)
	}
	return tools
}
