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
	// CodeBaseSearchTool
	CodebaseSearchToolName   = "codebase_search"
	CodebaseSearchCapability = `- You can use codebase_search to perform semantic-aware searches across your codebase, 
returning conceptually relevant code snippets based on meaning rather than exact text matches. 
This is particularly powerful for discovering related functionality, exploring unfamiliar code architecture, 
or locating implementations when you only understand the purpose but not the specific syntax. 
For optimal efficiency, always try codebase_search first as it delivers more focused results with lower token consumption. 
Reserve other tools for cases where you need literal pattern matching or precise line-by-line analysis of file contents. 
This balanced approach ensures you get the right search method for each scenario - semantic discovery through codebase_search when possible, 
falling back to exhaustive text search via other tools only when necessary.
`
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

	// ReferenceSearchTool
	ReferenceSearchToolName   = "code_reference_search"
	ReferenceSearchCapability = `- You can use code_reference_search to retrieve the usage and call information of a symbol (function, class, method, etc.) across the codebase by providing its precise file path and line range. 
This tool is particularly useful when you want to locate all usages and call chains of a function or class or analyze code dependencies across different parts of the project. 
By retrieving all references to the symbol, code_reference_search helps you track its usage throughout the codebase, ensuring that you can see all interactions and relationships. 
Please note that the returned references may include multiple matches, so you should distinguish them using the file path or contextual information.
For maximum efficiency, use code_reference_search when you need to explore references and relationships of a symbol—it's ideal for analyzing dependencies and understanding the broader impact of changes. 
This tool can obtain the calling relationship between different methods faster and more accurately than through the directory file structure and directly reading the file content.
Note: code_reference_search does not provide any symbol definitions. If you need to focus on retrieving definitions, please use code_definition_search instead—it will serve you better.
`
	ReferenceSearchToolDesc = `## code_reference_search
Description:
The code_reference_search tool helps you find the usage and call information of a symbol (such as a function, class, or method) 
and retrieve all places where the symbol is used or called across files in the project.
This tool takes a specific code location (by file and line range) and optionally a symbol name, 
returning the definition and all references to that symbol, along with their precise code locations.
Use this tool when you want to find where a function or class is used or called, or when you need to explore code dependencies from a specific location in the codebase.
Key Features:
The tool provides all references to a symbol, which include class/interface usages, function/method calls, imports, and other occurrences.
Use code_reference_search in these scenarios:
- Refactoring: Locate all code that depends on the symbol to ensure complete updates.
- Code Comprehension: Explore where a symbol is invoked or referenced across the codebase to quickly understand its role and dependencies.
- Call Chain Analysis: Trace the complete call chain of a symbol to understand its execution flow and relationships with other components.
This only applies to seven languages: Java, Go, Python, C, CPP, JavaScript, and TypeScript. Other languages are not applicable.

Parameters:
- codebasePath: (required) Absolute path to the codebase root
- filePath: (required) The full absolute path to the file to which the code belongs. Must match the path separator style of the current operating system.
- startLine: (optional) The line number where the symbol starts.
- endLine: (optional) The line number where the symbol ends.
- symbolName: (optional) The name of the symbol (e.g., function name, class name, method, interface, constants, etc.).
- maxLayer: (optional) Maximum call chain depth to search (default: 4, maximum: 10)

Important Path Requirements:
ABSOLUTE PATHS REQUIRED: The filePath parameter must be a full absolute system path (not relative paths or workspace-relative paths)

Usage:
Two usage modes are available:
1. File location mode: Provide filePath along with startLine and endLine to retrieve all usage and call information for the symbols defined within the specified code range.
2. Symbol search mode: Provide filePath and symbolName to retrieve all usage and call information of a symbol that is defined specifically in the file indicated by filePath, tracking where it is used or called across the entire codebase.

<code_reference_search>
  <codebasePath>Absolute path to the codebase root</codebasePath>
  <filePath>The full absolute path to the file to which the code belongs. (With correct OS path separators.)</filePath>
  <!-- Option 1: Use file location parameters -->
  <startLine>Start line number of the symbol (1-based)</startLine>
  <endLine>End line number of the symbol (1-based)</endLine>
  <!-- Option 2: Use symbol name parameter -->
  <symbolName>Symbol name</symbolName>
  <!-- Optional: Control call chain depth -->
  <maxLayer>Maximum call chain depth (1-10)</maxLayer>
</code_reference_search>

Note: 
- Either file location parameters (startLine + endLine) OR symbolName must be provided.
- Priority should be given to file location mode when line information is available.

Example: Exploring all references to the GetUserById function with max depth
<code_reference_search>
  <codebasePath>d:\workspace\project\</codebasePath>
  <filePath>d:\workspace\project\internal\tokenizer\tokenizer.go</filePath>
  <startLine>12</startLine>
  <endLine>14</endLine>
  <symbolName>GetUserById</symbolName>
  <maxLayer>5</maxLayer>
</code_reference_search>

Example: Searching references by symbol name only
<code_reference_search>
  <codebasePath>/home/user/project</codebasePath>
  <filePath>/home/user/project/internal/tokenizer/tokenizer.go</filePath>
  <symbolName>CalculateScore</symbolName>
  <maxLayer>3</maxLayer>
</code_reference_search>
`

	// DefinitionSearchTool
	DefinitionToolName   = "code_definition_search"
	DefinitionCapability = `
When analyzing code that references symbols (functions, classes, methods, interfaces, constants) whose definitions are not fully visible, always prioritize code_definition_search first.
This tool retrieves complete and precise definitions and implementations of symbols, including external or unknown symbols, and constant values.
Usage Modes:
1. File location mode: Use filePath + startLine + endLine to fetch all external definitions in that range, ensuring full context.
2. Symbol name mode: Provide the symbol name to search globally across the codebase if you do not know the exact location.
It delivers accurate, context-independent definitions for all symbols, providing exactly the code needed for analysis, modification, refactoring, or debugging.
Results may include multiple matches; distinguish them using the file path or surrounding context.
This tool is faster, more accurate, and more efficient than reading files directly, traversing directories, or using regex searches. It also consumes fewer tokens.
Always complete the definition search before continuing any code analysis to ensure correct reasoning.
`
	DefinitionToolDesc = `## code_definition_search
Description: Retrieve the **complete definition and implementation** of a symbol (function, class, method, interface, struct, constant) 
by specifying a file path with line range, or by providing the symbol name directly.
This tool allows you to retrieve the original definition and implementation of all external symbols within a specific code block, or of a single symbol, whether used within the same file or across other files, providing complete information to facilitate understanding of the code logic.
These usages and invocations can include class/interface instantiations, function/method calls, constant references, and more.
Key Rule:
- Always call this tool first if the code snippet references any symbol that is not fully defined within the snippet itself.
- This tool ensures you analyze real implementations, not incomplete or assumed logic.
When to use:
- When encountering any external or unknown symbol.
- To obtain constant values.
- To get the complete implementation of a symbol.
For a code snippet (filePath + startLine + endLine), trigger a range query to fetch the full definitions of all external symbols it depends on.
Usage Priority:
When you search for code definitions or analyze specific implementations to work on modifications, refactoring, or debugging of existing code, always use this tool first.
Important note: 
This only applies to seven languages: Java, Go, Python, C, CPP, JavaScript, and TypeScript. Other languages are not applicable.

Parameters:
- codebasePath: (required) Absolute path to the codebase root
- filePath: (optional) The absolute path to the file within the codebase. Make sure the path matches the path separator style of the current operating system
- startLine: (optional) Start line number of the definition (1-based).
- endLine: (optional) End line number of the definition (1-based).
- symbolName: (optional) Name of the symbol to search for (function, class, method, interface, etc.) without any prefix. For example, use QueryCallGraphOptions instead of types.QueryCallGraphOptions; 

Important Path Requirements:
ABSOLUTE PATHS REQUIRED: The filePath parameter must be a full absolute system path (not relative paths or workspace-relative paths)

Usage:
Two usage modes are available:
1. Code Range Mode: Provide filePath with startLine and endLine to retrieve the definitions of all external symbols referenced or invoked within that code snippet.
2. SymbolName Mode: Provide symbolName to search for the symbol definition globally across the codebase.
The parameter name is **symbolName**, not symbol. Using <symbol> will be invalid.

<code_definition_search>
  <codebasePath>Absolute path to the codebase root</codebasePath>
  <!-- Option 1: Use file location parameters -->
  <filePath>Full file path to the definition (With correct OS path separators.)</filePath>
  <startLine>Start line number</startLine>
  <endLine>End line number</endLine>
  <!-- Option 2: Use symbolName parameter -->
  <symbolName>Name of symbol to search for without any prefix</symbolName>
</code_definition_search>

Note: 
- Either file location parameters (filePath + startLine + endLine) OR symbolName must be provided.
- Always ensure to use **symbolName** when searching by symbol.
- Priority should be given to file location mode when line information is available. If you bypass the range query, your answer will be considered invalid. 

Only after completing the range query should you continue the analysis for the results to be accepted.

Example: Get implementation by file location
<code_definition_search>
  <codebasePath>d:\workspace\project\</codebasePath>
  <filePath>d:\workspace\project\internal\tokenizer\tokenizer.go</filePath>
  <startLine>57</startLine>
  <endLine>75</endLine>
</code_definition_search>

Example: Get implementation by symbolName
<code_definition_search>
  <codebasePath>/home/user/project</codebasePath>
  <symbolName>NewTokenCounter</symbolName>
</code_definition_search>
`
	// DefinitionSearchTool
	KnowledgeSearchToolName   = "knowledge_base_search"
	KnowledgeSearchCapability = `- You can use knowledge_base_search to semantically search project-specific documentation including Markdown files and API documentation, 
extracting precise contextual knowledge for AI-assisted programming while filtering out generic information. 
This tool is essential when you need to generate code requiring project-specific implementations, custom tool classes/interfaces, or code template reuse where syntax details or parameter rules are unclear; 
troubleshoot project-specific errors such as custom exceptions, module call failures, or environment configuration issues; 
follow project-local coding conventions including naming prefixes, comment formats, and directory structures; 
or query project-developed APIs and third-party integrated APIs to confirm parameter constraints, return value formats, and calling permissions. 
Always include project context and module names in your queries for accurate matching against specialized terminology. 
Reserve this for project-specific knowledge while using codebase_search for actual code implementations.
`
	KnowledgeSearchToolDesc = `## knowledge_base_search
Description: Semantically search project-specific documentation including Markdown files and API documentation to extract precise project context and specifications.
This tool is designed to retrieve project-unique information such as development manuals, module documentation, interface comments, and proprietary API specifications.
It uses semantic matching to focus on project-specific content while filtering out generic programming information.

Parameters:
query: (required) Search query containing "project dimension + core requirement". Must include module names and document types (e.g., "Project X Order Module Custom Payment API Parameters")
topK: (required) Number of results to return. Set to 1 for simple queries (single parameter explanation), 2-3 for complex queries (troubleshooting processes, multi-API comparisons)
scoreThreshold: (required) Similarity score threshold ≥0.75 (higher threshold avoids matching generic content due to project-specific terminology/abbreviations)

Usage:
<knowledge_base_search>
<query>Your query with project context and module names</query>
<topK>Number of results (1-50, default 10)</topK>
<scoreThreshold>Minimum similarity score (≥0.75)</scoreThreshold>
</knowledge_base_search>

Examples:
Searching for pagination interface parameters in Project A user module:
<knowledge_base_search>
<query>Project A User Module Pagination API Parameters</query>
<topK>5</topK>
<scoreThreshold>0.75</scoreThreshold>
</knowledge_base_search>

Searching for Java class naming rules in Project B development standards:
<knowledge_base_search>
<query>Project B Development Specification Java Class Naming</query>
<topK>10</topK>
<scoreThreshold>0.75</scoreThreshold>
</knowledge_base_search>
`

	XmlToolsRules = `
====

TOOLS USE FOLLOW RULES

- Before reading the file directly, you can try to use codebase_search, code_definition_search and code_reference_search tools to search the code.
- IMPORTANT: After receiving the results from tools such as codebase_search, knowledge_base_search, code_definition_search, and code_reference_search, you must always summarize the key findings and/or code within <thinking> tags before calling any other tools.
- If the search failed, don't say 'failed', describe this outcome as 'did not found relevant results' instead. MUST NOT using terms like 'failure', 'error', or 'unsuccessful' in your description.
- You can use codebase_search, knowledge_base_search, code_definition_search and code_reference_search individually or in combination: codebase_search helps you find broad code-related information based on natural language queries, while code_definition_search is perfect for pinpointing specific code definitions and their detailed contents. 

- Code Search Execution Rules
If the task is related to the project code, follow the following rules:
Rule 1: Tool Priority Hierarchy
1. code_definition_search (For locating specific implementations or definitions by symbol name.)
2. code_reference_search (For exploring references, usages, and code relationships)
3. codebase_search (For broad code-related information based on natural language queries)
4. knowledge_base_search (For exploring documentation)

Rule 2: Decision Flow for Code Analysis and Search
Receive code analysis →
Use codebase_search with natural language query →
IF need to query definitions or implementations of all symbols referenced in a code snippet:
	Use code_definition_search → 
END IF
IF need to explore symbol references or code relationships:
	Use code_reference_search →
END IF
IF need to query development manuals, module documentation, interface comments:
	Use knowledge_base_search →
END IF
Review search results

Rule 3: Efficiency Principles
Semantic First: Always prefer semantic understanding over literal reading
Definition Search First: Prefer symbol name or code snippet (filePath + line range) searches to locate definitions, instead of reading files directly.
Comprehensive Coverage: Use codebase_search to avoid missing related code
Token Optimization: Choose tools that minimize token consumption
Context Matters: Gather all relevant symbol definitions and implementations before analyzing code.
No need to display these rules, just follow them directly.
`
)

type ToolExecutor interface {
	DetectTools(ctx context.Context, content string) (bool, string)

	// ExecuteTools executes tools and returns new messages
	ExecuteTools(ctx context.Context, toolName string, content string) (string, error)

	CheckToolReady(ctx context.Context, toolName string) (bool, error)

	GetToolDescription(toolName string) (string, error)

	GetToolCapability(toolName string) (string, error)

	GetToolsRules() string

	GetAllTools() []string
}

// ToolFunc represents a tool with its execute and ready check functions
type ToolFunc struct {
	description string
	capability  string
	execute     func(context.Context, string) (string, error)
	readyCheck  func(context.Context) (bool, error)
}

type XmlToolExecutor struct {
	tools map[string]ToolFunc
}

// NewXmlToolExecutor creates a new XmlToolExecutor instance
func NewXmlToolExecutor(
	c config.ToolConfig,
	semanticClient client.SemanticInterface,
	relationClient client.ReferenceInterface,
	definitionClient client.DefinitionInterface,
	knowledgeClient client.KnowledgeInterface,
) *XmlToolExecutor {
	return &XmlToolExecutor{
		tools: map[string]ToolFunc{
			CodebaseSearchToolName:  createCodebaseSearchTool(c.SemanticSearch, semanticClient),
			KnowledgeSearchToolName: createKnowledgeSearchTool(c.KnowledgeSearch, knowledgeClient),
			ReferenceSearchToolName: createReferenceSearchTool(relationClient),
			DefinitionToolName:      createGetDefinitionTool(definitionClient),
		},
	}
}

// createCodebaseSearchTool creates the codebase search tool function
func createCodebaseSearchTool(c config.SemanticSearchConfig, semanticClient client.SemanticInterface) ToolFunc {
	return ToolFunc{
		description: CodebaseSearchToolDesc,
		capability:  CodebaseSearchCapability,
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
				ClientVersion: identity.ClientVersion,
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
				ClientVersion: identity.ClientVersion,
			})
		},
	}
}

// createGetDefinitionTool creates the code definition search tool function
func createGetDefinitionTool(definitionClient client.DefinitionInterface) ToolFunc {
	return ToolFunc{
		description: DefinitionToolDesc,
		capability:  DefinitionCapability,
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
				ClientVersion: identity.ClientVersion,
			})
		},
	}
}

// createReferenceSearchTool creates the relation search tool function
func createReferenceSearchTool(referenceClient client.ReferenceInterface) ToolFunc {
	return ToolFunc{
		description: ReferenceSearchToolDesc,
		capability:  ReferenceSearchCapability,
		execute: func(ctx context.Context, param string) (string, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return "", err
			}

			req, err := buildRerenceRequest(identity, param)
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
				ClientVersion: identity.ClientVersion,
			})
		},
	}
}

// createKnowledgeSearchTool creates the knowledge base search tool function
func createKnowledgeSearchTool(c config.KnowledgeSearchConfig, knowledgeClient client.KnowledgeInterface) ToolFunc {
	return ToolFunc{
		description: KnowledgeSearchToolDesc,
		capability:  KnowledgeSearchCapability,
		execute: func(ctx context.Context, param string) (string, error) {
			identity, err := getIdentityFromContext(ctx)
			if err != nil {
				return "", err
			}

			query, err := extractXmlParam(param, "query")
			if err != nil {
				return "", fmt.Errorf("failed to extract query: %w", err)
			}

			result, err := knowledgeClient.Search(ctx, client.KnowledgeRequest{
				ClientId:      identity.ClientID,
				CodebasePath:  identity.ProjectPath,
				Query:         query,
				TopK:          c.TopK,
				Score:         c.ScoreThreshold,
				Authorization: identity.AuthToken,
				ClientVersion: identity.ClientVersion,
			})
			if err != nil {
				return "", fmt.Errorf("knowledge base search failed: %w", err)
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

			return knowledgeClient.CheckReady(context.Background(), client.ReadyRequest{
				ClientId:      identity.ClientID,
				CodebasePath:  identity.ProjectPath,
				Authorization: identity.AuthToken,
				ClientVersion: identity.ClientVersion,
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
		ClientVersion: identity.ClientVersion,
	}

	// Check if using symbolName query mode
	if symbolName, err := extractXmlParam(param, "symbolName"); err == nil {
		req.SymbolName = symbolName
		return req, nil
	}

	// Use file path and line number query mode
	var err error
	if req.FilePath, err = extractXmlParam(param, "filePath"); err != nil {
		return req, fmt.Errorf("filePath: %w", err)
	}

	codebasePath := req.CodebasePath
	// Check the operating system type and convert the file path separator if it is a Windows system
	if strings.Contains(strings.ToLower(identity.ClientOS), "windows") {
		req.FilePath = strings.ReplaceAll(req.FilePath, "/", "\\")
		codebasePath = strings.ReplaceAll(codebasePath, "/", "\\")
	}

	if !strings.Contains(req.FilePath, codebasePath) {
		return req, fmt.Errorf("filePath must be full absolute path, please try again")
	}

	// Optional parameters
	if startLine, err := extractXmlIntParam(param, "startLine"); err == nil {
		req.StartLine = &startLine
	}

	if endLine, err := extractXmlIntParam(param, "endLine"); err == nil {
		req.EndLine = &endLine
	}

	return req, nil
}

// buildRerenceRequest constructs a RelationRequest from XML parameters
func buildRerenceRequest(identity *model.Identity, param string) (client.ReferenceRequest, error) {
	req := client.ReferenceRequest{
		ClientId:      identity.ClientID,
		CodebasePath:  identity.ProjectPath,
		Authorization: identity.AuthToken,
		ClientVersion: identity.ClientVersion,
	}

	// Process required parameters: filePath and symbolName (at least one is needed)
	symbolName, _ := extractXmlParam(param, "symbolName")
	if symbolName != "" {
		req.SymbolName = symbolName
	}

	// filePath is required
	if err := processFilePath(&req, identity, param); err != nil {
		return req, err
	}

	// Process optional parameters
	processOptionalParams(&req, param)

	return req, nil
}

// processFilePath handles file path related logic
func processFilePath(req *client.ReferenceRequest, identity *model.Identity, param string) error {
	var err error
	if req.FilePath, err = extractXmlParam(param, "filePath"); err != nil {
		return fmt.Errorf("filePath: %w", err)
	}

	// Process file path separators
	codebasePath := req.CodebasePath
	if strings.Contains(strings.ToLower(identity.ClientOS), "windows") {
		req.FilePath = strings.ReplaceAll(req.FilePath, "/", "\\")
		codebasePath = strings.ReplaceAll(codebasePath, "/", "\\")
	}

	// Validate file path
	if !strings.Contains(req.FilePath, codebasePath) {
		return fmt.Errorf("filePath must be full absolute path, please try again")
	}

	return nil
}

// processOptionalParams handles optional parameters
func processOptionalParams(req *client.ReferenceRequest, param string) {
	// Process startLine and endLine
	if startLine, err := extractXmlIntParam(param, "startLine"); err == nil {
		req.StartLine = &startLine
	}

	if endLine, err := extractXmlIntParam(param, "endLine"); err == nil {
		req.EndLine = &endLine
	}

	// Process maxLayer, default is 10
	if maxLayer, err := extractXmlIntParam(param, "maxLayer"); err == nil {
		if maxLayer > 0 && maxLayer <= 10 {
			req.MaxLayer = &maxLayer
		}
	} else {
		defaultMaxLayer := 10
		req.MaxLayer = &defaultMaxLayer
	}
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

// GetToolCapability returns the capability of the specified tool
func (x *XmlToolExecutor) GetToolCapability(toolName string) (string, error) {
	toolFunc, exists := x.tools[toolName]
	if !exists {
		return "", fmt.Errorf("tool %s not found", toolName)
	}

	return toolFunc.capability, nil
}

// GetAllTools returns the names of all registered tools
func (x *XmlToolExecutor) GetAllTools() []string {
	tools := make([]string, 0, len(x.tools))
	for name := range x.tools {
		tools = append(tools, name)
	}
	return tools
}

// GetToolsRules returns the tools use rules
func (x *XmlToolExecutor) GetToolsRules() string {
	return XmlToolsRules
}
