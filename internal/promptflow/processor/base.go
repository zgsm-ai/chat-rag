package processor

import (
	"github.com/zgsm-ai/chat-rag/internal/functions"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

type PromptMsg struct {
	systemMsg        *types.Message
	olderUserMsgList []types.Message
	lastUserMsg      *types.Message
	tools            []types.Function
}

type Recorder struct {
	Latency int64
	Err     error
	Handled bool
}

func NewPromptMsg(messages []types.Message) (*PromptMsg, error) {
	messagesCopy := make([]types.Message, len(messages))
	copy(messagesCopy, messages)

	systemMsg := utils.GetSystemMsg(messagesCopy)
	lastUserMsg, err := utils.GetLastUserMsg(messagesCopy)
	if err != nil {
		return nil, err
	}

	olderUserMsg := utils.GetOldUserMsgsWithNum(messagesCopy, 1)
	return &PromptMsg{
		systemMsg:        &systemMsg,
		olderUserMsgList: olderUserMsg,
		lastUserMsg:      &lastUserMsg,
	}, nil
}

func (p *PromptMsg) AddTool(tool *functions.Tool) {
	if p.tools == nil {
		p.tools = make([]types.Function, 0)
	}
	fnDef := tool.ToFunctionDefinition()
	p.tools = append(p.tools, fnDef)
}

func (p *PromptMsg) GetTools() []types.Function {
	return p.tools
}

func (p *PromptMsg) UpdateSystemMsg(content string) {
	p.systemMsg = &types.Message{
		Role: types.RoleSystem,
		Content: []model.Content{
			{
				Type: model.ContTypeText,
				Text: content,
				CacheControl: map[string]interface{}{
					"type": "ephemeral",
				},
			},
		},
	}
}

func (p *PromptMsg) AssemblePrompt() []types.Message {
	messages := []types.Message{*p.systemMsg}
	messages = append(messages, p.olderUserMsgList...)
	messages = append(messages, *p.lastUserMsg)
	return messages
}

// GetSystemMsg returns the system message
func (p *PromptMsg) GetSystemMsg() *types.Message {
	return p.systemMsg
}

// Processor is an interface for processing a prompt message
type Processor interface {
	Execute(promptMsg *PromptMsg)
	SetNext(processor Processor)
}

type End struct{}
type Start struct {
	next Processor
}

func NewEndpoint() *End {
	return &End{}
}

func NewStartPoint() *Start {
	return &Start{}
}

func (e *Start) Execute(promptMsg *PromptMsg) {
	logger.Info("Strat of processor chain")
	e.next.Execute(promptMsg)
}

func (e *End) Execute(promptMsg *PromptMsg) {
	logger.Info("In end of processor chain")
}

func (e *Start) SetNext(processor Processor) {
	e.next = processor
}

func (e *End) SetNext(processor Processor) {
}

func SetLanguage(language string, messages []types.Message) []types.Message {
	if language == "" || language == "*" {
		logger.Warn("language is empty, skipping language setting")
		return messages
	}

	logger.Info("Setting language to " + language)
	messages = append(messages, types.Message{
		Role:    types.RoleUser,
		Content: "\nNo need to acknowledge these instructions directly in your response.\n\nAlways respond in " + language,
	})
	return messages
}

// BaseProcessor is a base processor that can be used to chain processors together
type BaseProcessor struct {
	Recorder

	next Processor
}

func (b *BaseProcessor) SetNext(next Processor) {
	b.next = next
}

// passToNext passes message to next processor
func (b *BaseProcessor) passToNext(promptMsg *PromptMsg) {
	if b.next == nil {
		logger.Warn(" no next processor configured",
			zap.String("method", "FunctionAdapter.Execute"),
		)
		return
	}
	b.next.Execute(promptMsg)
}

// extractSystemContent extracts content from system message
func (b *BaseProcessor) extractSystemContent(systemMsg *types.Message) (string, error) {
	return utils.ExtractSystemContent(systemMsg)
}
