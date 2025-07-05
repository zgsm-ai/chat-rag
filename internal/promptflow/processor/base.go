package processor

import (
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

type PromptMsg struct {
	systemMsg        *types.Message
	olderUserMsgList []types.Message
	lastUserMsg      *types.Message
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

func (p *PromptMsg) AssemblePrompt() []types.Message {
	messages := []types.Message{*p.systemMsg}
	messages = append(messages, p.olderUserMsgList...)
	messages = append(messages, *p.lastUserMsg)
	return messages
}

type Processor interface {
	Execute(promptMsg *PromptMsg)
	SetNext(processor Processor)
}

type End struct{}

func NewEndpoint() *End {
	return &End{}
}

func (e *End) Execute(promptMsg *PromptMsg) {
	logger.Info("In end of processor chain")
}

func (e *End) SetNext(processor Processor) {
}

func SetLanguage(language string, messages []types.Message) []types.Message {
	if language == "" {
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
