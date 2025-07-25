package functions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// ToolExecutor 定义工具执行器接口
type ToolExecutor interface {
	DetectTools(ctx context.Context, content string) (bool, string)

	// ExecuteTools 执行工具并返回新消息
	ExecuteTools(
		ctx context.Context, toolName string, param string, messages []types.Message,
	) ([]types.Message, error)
}

// XmlToolExecutor 默认工具执行器实现
type XmlToolExecutor struct {
	tools map[string]func(context.Context, string) (string, error)
}

func NewXmlToolExecutor() *XmlToolExecutor {
	// return &XmlToolExecutor{
	// 	tools: make(map[string]func(context.Context, string) (string, error)),
	// }

	toolExecutor := &XmlToolExecutor{
		tools: map[string]func(context.Context, string) (string, error){
			"codebase_search": func(ctx context.Context, param string) (string, error) {
				fmt.Printf("==> Executing codebase_search tool, param: %s\n", param)

				time.Sleep(5 * time.Second)
				return "codebase_search工具当前不可用，使用其他工具", nil
			},
		},
	}

	return toolExecutor
}

// DetectTools 仅检测是否包含工具调用并提取工具信息
func (e *XmlToolExecutor) DetectTools(
	ctx context.Context,
	content string,
) (bool, string) { // 返回：是否检测到工具, 工具名, 参数, 错误
	for toolName := range e.tools {
		if strings.Contains(content, "<"+toolName+">") {
			return true, toolName
		}
	}
	return false, ""
}

// ExecuteTools 执行指定工具并构建新消息
func (e *XmlToolExecutor) ExecuteTools(
	ctx context.Context,
	toolName string,
	content string,
	messages []types.Message,
) ([]types.Message, error) {
	// 获取工具函数
	toolFunc, exists := e.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("tool %s not registered", toolName)
	}

	start := strings.Index(content, "<"+toolName+">")
	end := strings.Index(content, "</"+toolName+">")
	if end == -1 {
		newMessages := append(messages,
			types.Message{
				Role:    types.RoleAssistant,
				Content: fmt.Sprintf("<%s>%s</%s>", toolName, content, toolName),
			},
			types.Message{
				Role: types.RoleUser,
				Content: []model.Content{
					{
						Type: model.ContTypeText,
						Text: fmt.Sprintf("[%s] Result Error:", toolName),
					}, {
						Type: model.ContTypeText,
						Text: "can not extract param",
					},
				},
			},
		)

		return newMessages, nil
	}

	param := content[start+len(toolName)+2 : end]

	// 执行工具
	result, err := toolFunc(ctx, param)
	if err != nil {
		return nil, fmt.Errorf("tool %s execution failed: %w", toolName, err)
	}

	fmt.Printf("==> tool execute succeed: %s\n", result)

	// 构建新的消息列表
	newMessages := append(messages,
		types.Message{
			Role:    types.RoleAssistant,
			Content: fmt.Sprintf("<%s>%s</%s>", toolName, param, toolName),
		},
		types.Message{
			Role: types.RoleUser,
			Content: []model.Content{
				{
					Type: model.ContTypeText,
					Text: fmt.Sprintf("[%s] Result:", toolName),
				}, {
					Type: model.ContTypeText,
					Text: result,
				},
			},
		},
	)

	return newMessages, nil
}
