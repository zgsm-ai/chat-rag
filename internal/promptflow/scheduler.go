package promptflow

import (
	"context"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/ds"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/strategies"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"go.uber.org/zap"
)

// PromptArranger is the interface for processing chat prompts
type PromptArranger interface {
	Arrange(messages []types.Message) (*ds.ProcessedPrompt, error)
}

// NewPromptProcessor creates a new processor based on chat type
func NewPromptProcessor(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	promptMode types.PromptMode,
	identity *model.Identity,
) PromptArranger {
	switch promptMode {
	case types.Raw:
		logger.Info("Direct chat mode detected, using DirectProcessor")
		return &strategies.DirectProcessor{}

	case types.Cost, types.Performance, types.Balanced, types.Auto:
		fallthrough
	default:
		logger.Info("RAG processing mode activated",
			zap.String("mode", string(promptMode)),
		)
		ragProcessor, err := strategies.NewRagProcessor(ctx, svcCtx, identity)
		if err != nil {
			logger.Error("Failed new RAG processor, falling back to DirectProcessor",
				zap.Error(err),
			)
			return &strategies.DirectProcessor{}
		}
		return ragProcessor
	}
}
