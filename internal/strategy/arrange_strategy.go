package strategy

import (
	"context"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"go.uber.org/zap"
)

// ProcessedPrompt contains the result of prompt processing
type ProcessedPrompt struct {
	Messages        []types.Message      `json:"messages"`
	IsCompressed    bool                 `json:"is_compressed"`
	SemanticLatency int64                `json:"semantic_latency_ms"`
	SemanticContext *client.SemanticData `json:"semantic_context"`
	SummaryLatency  int64                `json:"summary_latency_ms"`
	SemanticErr     error                `json:"semantic_err"`
	SummaryErr      error                `json:"summary_err"`
}

// PromptProcessor is the interface for processing chat prompts
type PromptProcessor interface {
	Process(messages []types.Message) (*ProcessedPrompt, error)
}

// DirectProcessor directly passes through messages without processing
type DirectProcessor struct {
}

// Process implements the PromptProcessor interface for DirectProcessor
func (d *DirectProcessor) Process(messages []types.Message) (*ProcessedPrompt, error) {
	logger.Info("DirectProcessor process")
	return &ProcessedPrompt{
		Messages: messages,
	}, nil
}

// NewPromptProcessor creates a new processor based on chat type
func NewPromptProcessor(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	promptMode types.PromptMode,
	identity *types.Identity,
) PromptProcessor {
	switch promptMode {
	case types.Raw:
		logger.Info("Direct chat mode detected, using DirectProcessor")
		return &DirectProcessor{}

	case types.Cost, types.Performance, types.Balanced, types.Auto:
		fallthrough
	default:
		logger.Info("RAG processing mode activated",
			zap.String("mode", string(promptMode)),
		)
		ragProcessor, err := NewRagProcessor(ctx, svcCtx, identity)
		if err != nil {
			logger.Error("Failed new RAG processor, falling back to DirectProcessor",
				zap.Error(err),
			)
			return &DirectProcessor{}
		}
		return ragProcessor
	}
}
