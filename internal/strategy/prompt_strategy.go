package strategy

import (
	"context"
	"log"

	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// ProcessedPrompt contains the result of prompt processing
type ProcessedPrompt struct {
	Messages        []types.Message `json:"messages"`
	IsCompressed    bool            `json:"is_compressed"`
	SemanticLatency int64           `json:"semantic_latency_ms"`
	SemanticContext string          `json:"semantic_context"`
	SummaryLatency  int64           `json:"summary_latency_ms"`
	SemanticErr     error           `json:"semantic_err"`
	SummaryErr      error           `json:"summary_err"`
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
	log.Printf("[process] DirectProcessor process")
	return &ProcessedPrompt{
		Messages: messages,
	}, nil
}

// NewPromptProcessor creates a new processor based on chat type
func NewPromptProcessor(ctx context.Context, svcCtx *svc.ServiceContext, chatMode types.ChatMode, identity *types.Identity) PromptProcessor {
	switch chatMode {
	case types.Direct:
		log.Printf("[NewPromptProcessor] Direct chat mode detected, using DirectProcessor")
		return &DirectProcessor{}

	default:
		log.Printf("[NewPromptProcessor] RAG processing mode activated for type: %s", chatMode)
		ragProcessor, err := NewRagProcessor(ctx, svcCtx, identity)
		if err != nil {
			log.Printf("[NewPromptProcessor] Failed to initialize RAG processor, falling back to DirectProcessor. Error: %v", err)
			return &DirectProcessor{}
		}
		return ragProcessor
	}
}
