package strategy

import (
	"context"
	"fmt"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// PromptProcessor defines the interface for prompt processing strategies
type PromptProcessor interface {
	ProcessPrompt(ctx context.Context, req *types.ChatCompletionRequest) (*ProcessedPrompt, error)
}

// ProcessedPrompt contains the result of prompt processing
type ProcessedPrompt struct {
	Messages         []types.Message `json:"messages"`
	IsCompressed     bool            `json:"is_compressed"`
	OriginalTokens   int             `json:"original_tokens"`
	CompressedTokens int             `json:"compressed_tokens"`
	CompressionRatio float64         `json:"compression_ratio"`
	SemanticLatency  int64           `json:"semantic_latency_ms"`
	SummaryLatency   int64           `json:"summary_latency_ms"`
}

// DirectProcessor processes prompts without compression
type DirectProcessor struct{}

// NewDirectProcessor creates a new direct processor
func NewDirectProcessor() *DirectProcessor {
	return &DirectProcessor{}
}

// ProcessPrompt processes the prompt directly without compression
func (p *DirectProcessor) ProcessPrompt(ctx context.Context, req *types.ChatCompletionRequest) (*ProcessedPrompt, error) {
	return &ProcessedPrompt{
		Messages:         req.Messages,
		IsCompressed:     false,
		OriginalTokens:   0, // Will be calculated by caller
		CompressedTokens: 0,
		CompressionRatio: 1.0,
		SemanticLatency:  0,
		SummaryLatency:   0,
	}, nil
}

// CompressionProcessor processes prompts with RAG compression
type CompressionProcessor struct {
	semanticClient *client.SemanticClient
	llmClient      *client.LLMClient
	topK           int
}

// NewCompressionProcessor creates a new compression processor
func NewCompressionProcessor(semanticClient *client.SemanticClient, llmClient *client.LLMClient, topK int) *CompressionProcessor {
	return &CompressionProcessor{
		semanticClient: semanticClient,
		llmClient:      llmClient,
		topK:           topK,
	}
}

// ProcessPrompt processes the prompt with RAG compression
func (p *CompressionProcessor) ProcessPrompt(ctx context.Context, req *types.ChatCompletionRequest) (*ProcessedPrompt, error) {
	// Get the latest user message for semantic search
	var latestUserMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			latestUserMessage = req.Messages[i].Content
			break
		}
	}

	if latestUserMessage == "" {
		return nil, fmt.Errorf("no user message found for semantic search")
	}

	// Perform semantic search
	semanticReq := client.SemanticRequest{
		ClientId:    req.ClientId,
		ProjectPath: req.ProjectPath,
		Query:       latestUserMessage,
		TopK:        p.topK,
	}

	semanticResp, err := p.semanticClient.Search(ctx, semanticReq)
	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}

	// Build context from semantic results
	var contextParts []string
	for _, result := range semanticResp.Results {
		contextParts = append(contextParts, fmt.Sprintf("File: %s (Line %d)\n%s",
			result.FilePath, result.LineNumber, result.Content))
	}
	semanticContext := strings.Join(contextParts, "\n\n")

	// Build summary prompt
	var historyParts []string
	for _, msg := range req.Messages {
		if msg.Role != "system" {
			historyParts = append(historyParts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
		}
	}
	history := strings.Join(historyParts, "\n")

	summaryPrompt := fmt.Sprintf(`Please summarize the following information while preserving key details and context:

SEMANTIC CONTEXT:
%s

CONVERSATION HISTORY:
%s

CURRENT QUERY:
%s

Please provide a concise summary that maintains the essential information needed to answer the current query.`,
		semanticContext, history, latestUserMessage)

	// Generate summary using LLM
	summary, err := p.llmClient.SummarizeContent(ctx, summaryPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// Build final messages
	var finalMessages []types.Message

	// Add system message if exists
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			finalMessages = append(finalMessages, msg)
			break
		}
	}

	// Add summary as context
	finalMessages = append(finalMessages, types.Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Based on the codebase context and conversation history: %s", summary),
	})

	// Add latest user message
	finalMessages = append(finalMessages, types.Message{
		Role:    "user",
		Content: latestUserMessage,
	})

	return &ProcessedPrompt{
		Messages:         finalMessages,
		IsCompressed:     true,
		OriginalTokens:   0, // Will be calculated by caller
		CompressedTokens: 0, // Will be calculated by caller
		CompressionRatio: 0, // Will be calculated by caller
		SemanticLatency:  0, // Will be set by caller
		SummaryLatency:   0, // Will be set by caller
	}, nil
}

// PromptProcessorFactory creates prompt processors based on configuration
type PromptProcessorFactory struct {
	semanticClient *client.SemanticClient
	llmClient      *client.LLMClient
	topK           int
}

// NewPromptProcessorFactory creates a new factory
func NewPromptProcessorFactory(semanticClient *client.SemanticClient, llmClient *client.LLMClient, topK int) *PromptProcessorFactory {
	return &PromptProcessorFactory{
		semanticClient: semanticClient,
		llmClient:      llmClient,
		topK:           topK,
	}
}

// CreateProcessor creates a processor based on whether compression is needed
func (f *PromptProcessorFactory) CreateProcessor(needsCompression bool) PromptProcessor {
	if needsCompression {
		return NewCompressionProcessor(f.semanticClient, f.llmClient, f.topK)
	}
	return NewDirectProcessor()
}
