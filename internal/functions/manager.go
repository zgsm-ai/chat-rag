package functions

import (
	"context"
	"fmt"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

// ToolManager manages tools
type ToolManager struct {
	tools    map[string]*Tool
	executor *ToolExecutor
}

// NewToolManager creates a tool manager and loads tools
func NewToolManager(toolsPath string) *ToolManager {
	tm := &ToolManager{
		tools:    make(map[string]*Tool),
		executor: NewToolExecutor(),
	}

	// Load all tools during initialization
	if err := tm.LoadFromYAMLFile(toolsPath); err != nil {
		logger.Error("Failed to load function tools", zap.Error(err))
		return nil
	}

	return tm
}

func (m *ToolManager) LoadFromYAMLFile(path string) error {
	type Tools struct {
		Tools []*Tool
	}
	toolList, err := config.LoadYAML[Tools](path)
	if err != nil {
		return err
	}

	for _, tool := range toolList.Tools {
		m.tools[tool.Name] = tool
		logger.Info("Loaded tool:", zap.String("name", tool.Name))
	}
	return nil
}

// GetTool gets a tool
func (m *ToolManager) GetTool(name string) (*Tool, bool) {
	tool, exists := m.tools[name]
	return tool, exists
}

// GetAllTools gets all tools
func (m *ToolManager) GetAllTools() []*Tool {
	tools := make([]*Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ExecuteTool executes a tool
func (m *ToolManager) ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}) (*ExecutionResult, error) {
	tool, exists := m.GetTool(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}
	return m.executor.Execute(ctx, tool, params)
}
