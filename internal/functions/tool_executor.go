package functions

import (
	"context"
	"fmt"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

type ToolExecutor interface {
	DetectTools(ctx context.Context, content string) (bool, string)

	// ExecuteTools executes tools and returns new messages
	ExecuteTools(ctx context.Context, toolName string, content string) (string, error)
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
`
)

// AvailableTools defines all available tools for the system
var AvailableTools = []string{
	CodebaseSearchTool,
	// Add other tools here if needed
}

type XmlToolExecutor struct {
	tools map[string]func(context.Context, string) (string, error)
}

func NewXmlToolExecutor(c config.Config, semanticClient client.SemanticInterface) *XmlToolExecutor {
	toolExecutor := &XmlToolExecutor{
		tools: map[string]func(context.Context, string) (string, error){
			"codebase_search": func(ctx context.Context, param string) (string, error) {
				identity, exists := model.GetIdentityFromContext(ctx)
				if !exists {
					return "", fmt.Errorf("no identity found in context")
				}
				query, err := ExactXmlParam(param, "query")
				if err != nil {
					return "", fmt.Errorf("param<%s> extract failed: %w", "query", err)
				}

				result, err := semanticClient.Search(ctx, client.SemanticRequest{
					ClientId:      identity.ClientID,
					CodebasePath:  identity.ProjectPath,
					Query:         query,
					TopK:          c.TopK,
					Authorization: identity.AuthToken,
					Score:         c.SemanticScoreThreshold,
				})
				if err != nil {
					return "", fmt.Errorf("semantic client search error: %w", err)
				}

				jsonResult, err := utils.MarshalJSONWithoutEscapeHTML(result.Results)
				if err != nil {
					return "", fmt.Errorf("result json encode error: %w", err)
				}
				return jsonResult, nil
			},
		},
	}

	return toolExecutor
}

// DetectTools only detects if tool calls are included and extracts tool information
// Returns: whether tool is detected, tool name
func (e *XmlToolExecutor) DetectTools(ctx context.Context, content string) (bool, string) {
	for toolName := range e.tools {
		if strings.Contains(content, "<"+toolName+">") {
			return true, toolName
		}
	}
	return false, ""
}

// ExecuteTools executes the specified tool and constructs new messages
func (e *XmlToolExecutor) ExecuteTools(ctx context.Context, toolName string, content string) (string, error) {
	// Get tool function
	toolFunc, exists := e.tools[toolName]
	if !exists {
		return "", fmt.Errorf("tool %s not registered", toolName)
	}

	param, err := ExactXmlParam(content, toolName)
	if err != nil {
		return "", fmt.Errorf("can not extract param")
	}

	// Execute tool
	result, err := toolFunc(ctx, param)
	if err != nil {
		return "", fmt.Errorf("tool %s execution failed: %w", toolName, err)
	}

	logger.Info("tool execution succeeded",
		zap.String("tool", toolName), zap.Any("result", result))

	return result, nil
}

// ExactXmlParam extracts the value of a specific XML parameter from the content
func ExactXmlParam(content string, paramName string) (string, error) {
	start := strings.Index(content, "<"+paramName+">")
	end := strings.Index(content, "</"+paramName+">")
	if end == -1 {
		return "", fmt.Errorf("can not extract param")
	}

	param := content[start+len(paramName)+2 : end]
	return param, nil
}
