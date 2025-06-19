package processor

import (
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
}

func NewPromptMsg(messages []types.Message) (*PromptMsg, error) {
	systemMsg := utils.GetSystemMsg(messages)
	lastUserMsg, err := utils.GetLastUserMsg(messages)
	if err != nil {
		return nil, err
	}

	olderUserMsg := utils.GetOldUserMsgsWithNum(messages, 1)
	return &PromptMsg{
		systemMsg:        &systemMsg,
		olderUserMsgList: olderUserMsg,
		lastUserMsg:      &lastUserMsg,
	}, nil
}

type Processor interface {
	Execute(promptMsg *PromptMsg)
	SetNext(processor Processor)
}
