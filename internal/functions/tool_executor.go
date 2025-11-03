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
	ReferenceSearchToolName   = "search_references"
	ReferenceSearchCapability = `- You can use search_references to retrieve comprehensive usage and call information for functions and methods across the entire codebase. This tool is particularly useful when you need to locate all usages and trace reverse call chains (caller chains) of a function or method, or when analyzing code dependencies across different modules and files. Compared to manually navigating directory structures and reading file contents, this tool provides a significantly faster and more accurate way to understand calling relationships between different functions and methods.
    - For example, when asked to review code snippets, investigate bugs, or analyze code, you MUST use search_references to obtain the reverse call chain of a function or method to understand how its parameters are passed, validated, and propagated in higher-level calls, which is critical because bugs are often caused by incorrect upstream parameter passing rather than the function itself, and without checking callers you might miss that a security vulnerability exists in how upstream code passes unvalidated input or that performance issues stem from callers invoking the function too frequently in wrong contexts.
    - For example, when asked to refactor code, you must use search_references to obtain the reverse call chain of a function or method before making any changes to identify all call sites that need modification when changing function signatures, assess backward compatibility requirements, and understand different calling patterns to ensure the refactored version handles all cases, because refactoring without checking callers first will break existing functionality across potentially dozens of call sites.
`

	ReferenceSearchToolDesc = `## search_references
Description: Retrieves the reverse call chain (caller chain) for a specified function or method within the codebase.
Given the name of a function or method, this tool traces all other functions or methods that directly or indirectly invoke it, providing a clear and context-rich view of its upstream dependencies.
You can specify a lineRange to precisely locate the target function or method, improving both the accuracy and efficiency of call chain generation.
This helps developers understand how a function or method is used, its relationships, and its dependency paths across the codebase.

**IMPORTANT: This only applies to seven languages: Java, Go, Python, C, CPP, JavaScript, and TypeScript. Other languages are not applicable.

Parameters:
- filePath: (required) The path of the file where the function or method is defined (relative to workspace directory)
- maxLayer: (required) Maximum call chain depth to search (default: 4, maximum: 10)
- symbolName: (required) The name of the function or method 
- lineRange: (optional) The line range of the function or method definition in format "start-end" (1-based)

Usage:

<search_references>
  <filePath>path/to/file</filePath>
  <maxLayer>call chain depth (1-10)</maxLayer>
  <symbolName>symbol name</symbolName>
  <lineRange>start-end</lineRange>
</search_references>

Examples
1. Exploring reverse call chain of the queryCallGraphBySymbol function 
<search_references>
  <filePath>internal\service\indexer.go</filePath>
  <maxLayer>4</maxLayer>
  <symbolName>queryCallGraphBySymbol</symbolName>
</search_references>

2. Exploring reverse call chain of the queryCallGraphByLineRange function with lineRange:
<search_references>
  <filePath>internal\tokenizer\tokenizer.go</filePath>
  <maxLayer>5</maxLayer>
  <symbolName>queryCallGraphByLineRange</symbolName>
  <lineRange>20-75</lineRange>
</search_references>
`

	// DefinitionSearchTool
	DefinitionToolName   = "search_definitions"
	DefinitionCapability = `
- You can use the search_definitions tool to retrieve the complete definitions and implementations of one or more specified symbols (such as constants,functions, classes, methods, interfaces, and structs) by providing their symbol names. This is particularly useful when you need to understand the detailed structure and logic of a symbol, or to gather the definitions of referenced symbols to build a more complete context within the codebase. You may need to call this tool multiple times to examine different symbols relevant to your task.
    - For example, when asked to make edits, review code, investigate bugs, analyze code, or refactor code, you might first use search_definitions to obtain the full definitions and implementations of symbols referenced within the code, in order to supplement the context and fully understand the logic. Then, analyze the structure and behavior based on the gathered definitions. If understanding how a function or method is used throughout the codebase—including how its parameters are passed, validated, and propagated in higher-level calls—is valuable for tasks such as code review or refactoring, you can use search_references to find where it is referenced in other files or modules.
    - For example, when asked to retrieve symbol definitions, use this tool to obtain results more quickly, accurately, and with lower token cost.
    - For example, when asked to retrieve, find, show, or explain specific symbol definitions such as "show me the UserService class" or "what does the calculateTax function do", you should use search_definitions to obtain results more quickly, accurately, and with lower token cost compared to searching through files manually.
`

	DefinitionToolDesc = `## search_definitions
Description: Retrieve the complete definitions and implementations of one or more symbols defined within the project (not external libraries), or of symbols referenced within the code that you need to understand in context.
This tool allows you to quickly and accurately retrieve the original definitions and full implementations of all symbols—such as constants, structs, interfaces, functions, methods, and classes—as well as their values. It can also obtain the definitions and implementations of all external symbols within a specific code block, or of a single symbol, whether used within the same file or across other files, providing complete information to facilitate understanding of the code logic.
Compared with using the search_files tool or reading files directly, this tool is significantly more efficient, accurate, and token-friendly, allowing you to obtain results more quickly, accurately, and with lower token cost. It provides comprehensive information to help you fully understand the code logic and referenced symbols, making it the preferred method for retrieving symbol definitions.
Key Rule:
- Always call this tool first if the code snippet references any symbol that is not fully defined within the snippet itself.
- This tool ensures you analyze real implementations, not incomplete or assumed logic.
Note: 
1. This tool only applies to seven languages: Java, Go, Python, C, CPP, JavaScript, and TypeScript. Other languages are not applicable.
2. This tool is more efficient and uses fewer tokens than search_files tool or directly reading files to obtain symbol definitions.

Parameters:
- symbolNames: (required) One or more target symbol names to search for definitions. Separate each symbol name with a comma.

Usage:
<search_definitions>
  <symbolNames>SymbolName1,SymbolName2</symbolNames>
</search_definitions>


Examples:

1. Querying the definition of a single symbol:
<search_definitions>
  <symbolNames>QueryCallGraphOptions</symbolNames>
</search_definitions>

2. Querying multiple symbols (within the 8-symbol limit)
<search_definitions>
  <symbolNames>countFilesAndSize,RelationNode,defaultCacheCapacity</symbolNames>
</search_definitions>

IMPORTANT: You MUST follow this Efficient Symbol Defini Definition Strategy:
- You MUST query all related symbols in one go whenever possible (up to 8 symbols per operation).
- You MUST NOT split the request into multiple queries if they can be searched in a single batch operation.
- You MUST obtain all necessary context before analyzing or modifying code
- You MUST obtain complete definition information for each referenced symbol
- You MUST prioritize the most critical symbols first when querying multiple symbols
- You MUST write each symbol name in PLAIN form (e.g., types.QueryCallGraphOptions → QueryCallGraphOptions), omitting any package, namespace, or class prefixes.
- You MUST use subsequent queries for additional symbols if more than 8 need to be analyzed

**When to Use:**
- You need to check a symbol's full definition before editing, reviewing, or refactoring code.
- You need to analyze a symbol's structure, logic (across files/modules) and require its definition.
- You're troubleshooting bugs or security issues and need to understand a symbol's implementation via its definition.
- You plan to modify a function, method, class, or constant and need its definition to assess impact.
- You're building code context (e.g., understanding code logic) and need definitions of referenced symbols.

**When NOT to Use:**
- The task is minor code editing that doesn't require a symbol's complete definition.
- The request is exploratory or informational, with no specific symbol to look up the definition for.
- You don't have a specific symbol name to look up its definition (the tool relies on a clear symbol name to work).
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
- Don't use search_files to search for symbol definitions or call chains. It is highly inefficient, consumes excessive tokens, and returns imprecise results.
- Before reading the file directly, you can try to use codebase_search, search_definitions and search_references tools to search the code.
- IMPORTANT: After receiving the results from tools such as codebase_search, knowledge_base_search, search_definitions, and search_references, you must always summarize the key findings and/or code within <thinking> tags before calling any other tools.
- If the search failed, don't say "failed", describe this outcome as "did not find relevant results" instead.
- You MUST NOT use terms like "failure", "error", "context deadline exceeded", or "unsuccessful" in your description — even if the tool times out or returns no data. Always summarize the result neutrally (e.g., "no matching symbol definitions were found") instead of exposing internal errors.
- You can use codebase_search, knowledge_base_search, search_definitions and search_references individually or in combination: codebase_search helps you find broad code-related information based on natural language queries, search_definitions is used to retrieve symbol definitions and their detailed contents, and search_references helps you locate symbol references and call relationships within the code.
- When you need to retrieve a symbol's definition, first use the search_definitions tool. This tool allows you to efficiently obtain the full definition and implementation of functions, methods, classes, interfaces, structs, or constants across the project. search_definitions is precise and consumes fewer tokens than manually searching files, providing complete and context-rich results.
- When analyzing a function or method's usage across the project, first use search_references to obtain its reverse call chain. This tool allows you to efficiently locate all places where the symbol is called or referenced, including parameter propagation and higher-level dependencies. This is especially useful for bug investigation, performance analysis, security checks, and refactoring, as issues often arise from incorrect or frequent calls upstream rather than the function itself. 
- *IMPORTANT*: The search_references tool may return call chains that include functions with the same name. You must use context and the provided information to determine and output the most accurate call chain.
- *IMPORTANT*: The search_definitions tool may return multiple definitions for symbols with the same name. You must use context and the provided information to determine and output the most accurate symbol definition.

**Code Search Execution Rules**
If the task is related to the project code, follow the following rules:
Rule 1: Tool Priority Hierarchy
1. search_definitions (Best for obtaining the full definition or implementation of symbols such as functions, methods, classes, interfaces, structs, or constants defined within the project.)
2. search_references (Best for understanding how functions or methods are used, called, or propagated in higher-level logic.)
3. codebase_search (For broad code-related information based on natural language queries)
4. knowledge_base_search (For exploring documentation)

Rule 2: Decision Flow for Code Analysis and Search
Receive code analysis →
Use codebase_search with natural language query →
IF need to query definitions or implementations of one or more symbols by name:
	Use search_definitions → 
END IF
IF need to explore function or method reverse call chains or dependencies:
	Use search_references →
END IF
IF need to query development manuals, module documentation, interface comments:
	Use knowledge_base_search →
END IF
Review search results

Rule 3: Efficiency Principles
Semantic First: Always prefer semantic understanding over literal reading
Comprehensive Coverage: Use codebase_search to avoid missing related code
Token Optimization: Choose tools that minimize token consumption
Context Matters: Gather full context before analyzing the code and use the most efficient tool for the task.
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

	// Check if using symbolNames query mode
	if symbolNames, err := extractXmlParam(param, "symbolNames"); err == nil {
		req.SymbolNames = symbolNames
		return req, nil
	}

	// Use file path and line number query mode
	var err error
	if req.FilePath, err = extractXmlParam(param, "filePath"); err != nil {
		return req, fmt.Errorf("filePath: %w", err)
	}

	// Check the operating system type and convert the file path separator if it is a Windows system
	if strings.Contains(strings.ToLower(identity.ClientOS), "windows") {
		req.FilePath = strings.ReplaceAll(req.FilePath, "/", "\\")
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
	if strings.Contains(strings.ToLower(identity.ClientOS), "windows") {
		req.FilePath = strings.ReplaceAll(req.FilePath, "/", "\\")
	}

	return nil
}

// processOptionalParams handles optional parameters
func processOptionalParams(req *client.ReferenceRequest, param string) {
	// Process lineRange
	if lineRange, err := extractXmlParam(param, "lineRange"); err == nil {
		req.LineRange = lineRange
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
