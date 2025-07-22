package ds

import (
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// ProcessedPrompt contains the result of prompt processing
type ProcessedPrompt struct {
	Messages               []types.Message      `json:"messages"`
	SemanticLatency        int64                `json:"semantic_latency_ms"`
	SemanticContext        *client.SemanticData `json:"semantic_context"`
	SummaryLatency         int64                `json:"summary_latency_ms"`
	SemanticErr            error                `json:"semantic_err"`
	SummaryErr             error                `json:"summary_err"`
	IsUserPromptCompressed bool                 `json:"is_user_prompt_compressed"`
	Tools                  []types.Function     `json:"tools"`
}
