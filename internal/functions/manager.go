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
	tools       map[string]*Tool
	ideTools    []string
	serverTools []string
	executor    *ToolExecutor
}

// NewToolManager creates a tool manager and loads tools
func NewToolManager(toolsPath string) *ToolManager {
	tm := &ToolManager{
		tools:       make(map[string]*Tool),
		executor:    NewToolExecutor(),
		ideTools:    make([]string, 0),
		serverTools: make([]string, 0),
	}

	// Load all tools during initialization
	if err := tm.LoadFromYAMLFile(toolsPath); err != nil {
		logger.Error("Failed to load function tools", zap.Error(err))
		return nil
	}

	for _, tool := range tm.tools {
		if tool.Type == ToolTypeIDE {
			tm.ideTools = append(tm.ideTools, tool.Name)
		}
		if tool.Type == ToolTypeServer {
			tm.serverTools = append(tm.serverTools, tool.Name)
		}
	}

	logger.Info("ideTools", zap.Any("tools", tm.ideTools), zap.Int("nums", len(tm.ideTools)))
	logger.Info("serverTools", zap.Any("tools", tm.serverTools), zap.Int("nums", len(tm.serverTools)))
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

// GetClientTools 获取所有客户端工具名称列表
func (m *ToolManager) GetClientTools() []string {
	return m.ideTools
}

// GetServerTools 获取所有服务端工具名称列表
func (m *ToolManager) GetServerTools() []string {
	return m.serverTools
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
