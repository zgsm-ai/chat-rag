package strategies

import (
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/ds"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// DirectProcessor directly passes through messages without processing
type DirectProcessor struct {
}

// Arrange implements the PromptProcessor interface for DirectProcessor
func (d *DirectProcessor) Arrange(messages []types.Message) (*ds.ProcessedPrompt, error) {
	logger.Info("DirectProcessor process")
	return &ds.ProcessedPrompt{
		Messages: messages,
	}, nil
}
