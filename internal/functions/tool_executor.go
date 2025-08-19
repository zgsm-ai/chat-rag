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
Description: Find files most relevant to the search query.
This is a semantic search tool, so the query should ask for something semantically matching what is needed.
If it makes sense to only search in a particular directory, please specify it in the path parameter.
Unless there is a clear reason to use your own search query, please just reuse the user's exact query with their wording.
Their exact wording/phrasing can often be helpful for the semantic search query. 
Keeping the same exact question format can also be helpful.
IMPORTANT: Queries MUST be in English. Translate non-English queries before searching.
When you need to search for relevant codes, use this tool first.

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

	ReferenceSearchToolName = "code_reference_search"
	ReferenceSearchToolDesc = `## code_reference_search
Description:
Find the definition of a symbol and retrieve its **reference relationships**.
This tool takes a code location (by file and range) and optionally a symbol name, and returns its definition and all references (across files) with their precise code locations.
Use this tool when the user wants to understand **how a function/class is used or called**, or **explore code dependencies** from a specific location.
You MUST specify the file path and the exact code range (line and column positions) to locate the target symbol.
You can optionally provide a symbol name if it's known (e.g., function name), and enable content fetching if needed.

Parameters:
- filePath: (required) Relative path to the file where the symbol is located (e.g., src/utils/math.go).
- startLine: (required) The line number where the symbol starts.
- startColumn: (required) The column number where the symbol starts.
- endLine: (required) The line number where the symbol ends.
- symbolName: (optional) The name of the symbol (e.g., function name, class name). Use this only if you're confident about the symbol.

Usage:
<code_reference_search>
  <filePath>Relative path to the file containing the symbol</filePath>
  <startLine>Start line number of the symbol (1-based)</startLine>
  <endLine>End line number of the symbol (1-based)</endLine>
  <symbolName>Symbol name (optional)</symbolName>
</code_reference_search>


Example: Exploring all references to the GetUserById function
<code_reference_search>
  <filePath>src/services/user_service.go</filePath>
  <startLine>12</startLine>
  <endLine>14</endLine>
  <symbolName>GetUserById</symbolName>
</code_reference_search>
`

	DefinitionToolName = "code_definition_search"
	DefinitionToolDesc = `## code_definition_search
Description:
Retrieve the full definition implementation of a symbol (function, class, method, interface, etc.) within a specific range of lines in a code file. 
This tool allows you to search for the original definition content of code that is referenced elsewhere (either within the same file or in other files across the project). 
These references can include class/interface references, function/method calls, and more.
Usage Priority:
When you need to search for code definitions or analyze specific implementations, always use this tool first. 
It efficiently retrieves the precise definition and its details, helping you to avoid unnecessary navigation or additional steps.

Parameters:
- codebasePath: (required) Absolute path to the codebase root
- filePath: (required) Full path to the file within the codebase. Must match the path separator style of the current operating system.
- startLine: (required) Start line number of the definition (1-based).
- endLine: (required) End line number of the definition (1-based).

Usage:
<code_definition_search>
  <codebasePath>Absolute path to the codebase root</codebasePath>
  <filePath>Full file path to the definition (With correct OS path separators.)</filePath>
  <startLine>Start line number (required)</startLine>
  <endLine>End line number (required)</endLine>
</code_definition_search>

Example: Get the implementation of NewTokenCounter(Windows) - NOTE BACKSLASHES
<code_definition_search>
  <codebasePath>d:\workspace\project\</codebasePath>
  <filePath>d:\workspace\project\internal\tokenizer\tokenizer.go</filePath>
  <startLine>57</startLine>
  <endLine>75</endLine>
</code_definition_search>

Example: Get the implementation of NewTokenCounter(Linux) - NOTE FORWARD SLASHES
<code_definition_search>
  <codebasePath>/home/user/project</codebasePath>
  <filePath>/home/user/project/internal/tokenizer/tokenizer.go</filePath>
  <startLine>57</startLine>
  <endLine>75</endLine>
</code_definition_search>
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
	relationClient client.ReferenceInterface,
	definitionClient client.DefinitionInterface,
) *XmlToolExecutor {
	return &XmlToolExecutor{
		tools: map[string]ToolFunc{
			CodebaseSearchToolName: createCodebaseSearchTool(c, semanticClient),
			// RelationSearchToolName: createRelationSearchTool(relationClient),
			DefinitionToolName: createGetDefinitionTool(definitionClient),
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

			return result, nil
		},
		readyCheck: func(ctx context.Context) (bool, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return false, err
			}
			if identity.ClientID == "" {
				return false, fmt.Errorf("get none clientId")
			}

			return semanticClient.CheckReady(context.Background(), client.ReadyRequest{
				ClientId:      identity.ClientID,
				CodebasePath:  identity.ProjectPath,
				Authorization: identity.AuthToken,
			})
		},
	}
}

// createGetDefinitionTool creates the code definition search tool function
func createGetDefinitionTool(definitionClient client.DefinitionInterface) ToolFunc {
	return ToolFunc{
		description: DefinitionToolDesc,
		execute: func(ctx context.Context, param string) (string, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return "", err
			}

			req, err := buildDefinitionRequest(identity, param)
			if err != nil {
				return "", fmt.Errorf("failed to build request: %w", err)
			}

			result, err := definitionClient.Search(ctx, req)
			if err != nil {
				return "", fmt.Errorf("code definition search failed: %w", err)
			}

			return result, nil
		},
		readyCheck: func(ctx context.Context) (bool, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return false, err
			}
			if identity.ClientID == "" {
				return false, fmt.Errorf("get none clientId")
			}

			return definitionClient.CheckReady(context.Background(), client.ReadyRequest{
				ClientId:      identity.ClientID,
				CodebasePath:  identity.ProjectPath,
				Authorization: identity.AuthToken,
			})
		},
	}
}

// createReferenceSearchTool creates the relation search tool function
func createReferenceSearchTool(referenceClient client.ReferenceInterface) ToolFunc {
	return ToolFunc{
		description: ReferenceSearchToolDesc,
		execute: func(ctx context.Context, param string) (string, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return "", err
			}

			req, err := buildRelationRequest(identity, param)
			if err != nil {
				return "", fmt.Errorf("failed to build request: %w", err)
			}

			result, err := referenceClient.Search(ctx, req)
			if err != nil {
				return "", fmt.Errorf("relation search failed: %w", err)
			}

			return utils.MarshalJSONWithoutEscapeHTML(result)
		},
		readyCheck: func(ctx context.Context) (bool, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return false, err
			}
			if identity.ClientID == "" {
				return false, fmt.Errorf("get none clientId")
			}

			return referenceClient.CheckReady(context.Background(), client.ReadyRequest{
				ClientId:      identity.ClientID,
				CodebasePath:  identity.ProjectPath,
				Authorization: identity.AuthToken,
			})
		},
	}
}

// buildDefinitionRequest constructs a DefinitionRequest from XML parameters
func buildDefinitionRequest(identity *model.Identity, param string) (client.DefinitionRequest, error) {
	req := client.DefinitionRequest{
		ClientId:      identity.ClientID,
		CodebasePath:  identity.ProjectPath,
		Authorization: identity.AuthToken,
	}

	var err error
	if req.FilePath, err = extractXmlParam(param, "filePath"); err != nil {
		return req, fmt.Errorf("filePath: %w", err)
	}

	// Check the operating system type and convert the file path separator if it is a Windows system
	if strings.Contains(strings.ToLower(identity.ClientOS), "windows") {
		req.FilePath = strings.ReplaceAll(req.FilePath, "/", "\\")
	}

	// Optional parameters
	if startLine, err := extractXmlIntParam(param, "startLine"); err == nil {
		req.StartLine = &startLine
	}

	if endLine, err := extractXmlIntParam(param, "endLine"); err == nil {
		req.EndLine = &endLine
	}

	if codeSnippet, err := extractXmlParam(param, "codeSnippet"); err == nil {
		req.CodeSnippet = codeSnippet
	}

	return req, nil
}

// buildRelationRequest constructs a RelationRequest from XML parameters
func buildRelationRequest(identity *model.Identity, param string) (client.ReferenceRequest, error) {
	req := client.ReferenceRequest{
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

	paramValue := content[start+len(startTag) : end]

	// Check and replace double backslashes with single backslashes to conform to Windows path format
	paramValue = strings.ReplaceAll(paramValue, "\\\\", "\\")

	return paramValue, nil
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
