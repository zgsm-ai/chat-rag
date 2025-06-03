package strategy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
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
	semanticClient   *client.SemanticClient
	summaryProcessor *SummaryProcessor
	topK             int
}

// NewCompressionProcessor creates a new compression processor
func NewCompressionProcessor(semanticClient *client.SemanticClient, llmClient *client.LLMClient, topK int) *CompressionProcessor {
	return &CompressionProcessor{
		semanticClient:   semanticClient,
		summaryProcessor: NewSummaryProcessor(llmClient),
		topK:             topK,
	}
}

// ProcessPrompt processes the prompt with RAG compression
func (p *CompressionProcessor) ProcessPrompt(ctx context.Context, req *types.ChatCompletionRequest) (*ProcessedPrompt, error) {
	// Get the latest user message for semantic search
	var latestUserMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			latestUserMessage = utils.GetContentAsString(req.Messages[i].Content)
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

	// Build context from semantic results
	var contextParts []string
	var semanticContext string

	if err != nil {
		// If semantic search fails, continue without context
		log.Printf("Semantic search failed: %v", err)
		semanticContext = ""
	} else {
		for _, result := range semanticResp.Results {
			contextParts = append(contextParts, fmt.Sprintf("File: %s (Line %d)\n%s",
				result.FilePath, result.LineNumber, result.Content))
		}
		semanticContext = strings.Join(contextParts, "\n\n")
	}

	// Get messages to summarize (exclude system messages and last user message)
	var messagesToSummarize []types.Message
	for i := 0; i < len(req.Messages); i++ {
		// Skip system messages and the last user message
		if req.Messages[i].Role == "system" ||
			(req.Messages[i].Role == "user" && i >= len(req.Messages)-1) {
			continue
		}
		messagesToSummarize = append(messagesToSummarize, req.Messages[i])
	}

	summary, err := p.summaryProcessor.GenerateUserPromptSummary(ctx, semanticContext, messagesToSummarize)
	if err != nil {
		log.Printf("Failed to generate summary: %v", err)
		// On error, proceed with original messages
		return &ProcessedPrompt{
			Messages:         req.Messages,
			IsCompressed:     false,
			OriginalTokens:   0, // Will be calculated by caller
			CompressedTokens: 0, // Will be calculated by caller
			CompressionRatio: 1.0,
			SemanticLatency:  0,
			SummaryLatency:   0,
		}, nil
	}

	// Build final messages
	finalMessages := p.summaryProcessor.BuildUserSummaryMessages(ctx, req.Messages, summary)

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
	semanticClient     *client.SemanticClient
	summaryModelClient *client.LLMClient
	topK               int
}

// NewPromptProcessorFactory creates a new factory
func NewPromptProcessorFactory(semanticClient *client.SemanticClient, summaryModelClient *client.LLMClient, topK int) *PromptProcessorFactory {
	return &PromptProcessorFactory{
		semanticClient:     semanticClient,
		summaryModelClient: summaryModelClient,
		topK:               topK,
	}
}

// CreateProcessor creates a processor based on whether compression is needed
func (f *PromptProcessorFactory) CreateProcessor(needsCompression bool, headers *http.Header) PromptProcessor {
	if needsCompression {
		f.summaryModelClient.SetHeaders(headers)
		return NewCompressionProcessor(f.semanticClient, f.summaryModelClient, f.topK)
	}
	return NewDirectProcessor()
}
