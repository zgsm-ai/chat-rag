package strategy

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

// ProcessedPrompt contains the result of prompt processing
type ProcessedPrompt struct {
	Messages        []types.Message `json:"messages"`
	IsCompressed    bool            `json:"is_compressed"`
	SemanticLatency int64           `json:"semantic_latency_ms"`
	SemanticContext string          `json:"semantic_context"`
	SummaryLatency  int64           `json:"summary_latency_ms"`
}

// CompressionProcessor processes prompts with RAG compression
type CompressionProcessor struct {
	semanticClient   *client.SemanticClient
	summaryProcessor *SummaryProcessor
	tokenCounter     *utils.TokenCounter
	config           config.Config
}

// NewCompressionProcessor creates a new compression processor
func NewCompressionProcessor(svcCtx *svc.ServiceContext) (*CompressionProcessor, error) {
	llmClient, err := client.NewLLMClient(svcCtx.Config.LLMEndpoint, svcCtx.Config.SummaryModel)
	if err != nil {
		return nil, err
	}

	llmClient.SetHeaders(svcCtx.ReqCtx.Headers)
	return &CompressionProcessor{
		semanticClient:   client.NewSemanticClient(svcCtx.Config.SemanticApiEndpoint),
		summaryProcessor: NewSummaryProcessor(svcCtx.Config.SystemPromptSplitter, llmClient),
		config:           svcCtx.Config,
		tokenCounter:     svcCtx.TokenCounter,
	}, nil
}

// searchSemanticContext performs semantic search and constructs context string
func (p *CompressionProcessor) searchSemanticContext(
	ctx context.Context,
	req *types.ChatCompletionRequest,
	query string,
) string {
	// Prepare semantic request
	semanticReq := client.SemanticRequest{
		ClientId:    req.ClientId,
		ProjectPath: req.ProjectPath,
		Query:       query,
		TopK:        p.config.TopK,
	}

	// Execute search
	semanticResp, err := p.semanticClient.Search(ctx, semanticReq)
	if err != nil {
		log.Printf("[buildSemanticContext] Semantic search failed: %v", err)
		return ""
	}

	// Build context string from results
	var contextParts []string
	for _, result := range semanticResp.Results {
		contextParts = append(contextParts, fmt.Sprintf("File: %s (Line %d)\n%s",
			result.FilePath, result.LineNumber, result.Content))
	}

	semanticContext := strings.Join(contextParts, "\n\n")
	log.Printf("[buildSemanticContext] Searched semantic context: %s", semanticContext)

	return semanticContext
}

// ReplaceSystemMessages 替换消息列表中的系统消息
// processedSystemMsg: 处理后的系统消息（nil表示不移除系统消息）
// messages: 原始消息列表
func (p *CompressionProcessor) replaceSysMsgWithCompressed(messages []types.Message) []types.Message {
	var processedMsgs []types.Message
	var hasSystem bool

	for _, msg := range messages {
		if msg.Role == "system" {
			processedMsgs = append(processedMsgs, p.summaryProcessor.processSystemMessageWithCache(msg))
			hasSystem = true
		} else {
			processedMsgs = append(processedMsgs, msg)
		}
	}

	// 如果没有找到系统消息，返回原始消息列表
	if !hasSystem {
		return messages
	}
	return processedMsgs
}

// ProcessPrompt processes the prompt with RAG compression
func (p *CompressionProcessor) ProcessPrompt(ctx context.Context, req *types.ChatCompletionRequest, needsCompressUserMsg bool) (*ProcessedPrompt, error) {
	var semanticLatency, summaryLatency int64

	// Get the latest user message for semantic search
	latestUserMessage, err := utils.GetLatestUserMsg(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("no user message found for semantic search: %w", err)
	}

	// Record start time for semantic search
	semanticStart := time.Now()
	semanticContext := p.searchSemanticContext(ctx, req, latestUserMessage)
	semanticLatency = time.Since(semanticStart).Milliseconds()

	// Replace system messages with compressed messages
	replacedSystemMsgs := p.replaceSysMsgWithCompressed(req.Messages)

	if !needsCompressUserMsg {
		log.Printf("[ProcessPrompt] No need to compress user message")
		return &ProcessedPrompt{
			Messages:        replacedSystemMsgs,
			IsCompressed:    false,
			SemanticLatency: semanticLatency,
			SemanticContext: semanticContext,
			SummaryLatency:  0,
		}, nil
	}

	log.Printf("[ProcessPrompt] start compress user message")
	// Record start time for summary process
	summaryStart := time.Now()

	// Get messages to summarize (exclude system messages and num-th user message)
	messagesToSummarize := utils.GetOldUserMsgsWithNum(req.Messages, p.config.RecentUserMsgUsedNums)
	messagesToSummarize = p.trimMessagesToTokenThreshold(semanticContext, messagesToSummarize)

	summary, err := p.summaryProcessor.GenerateUserPromptSummary(ctx, semanticContext, messagesToSummarize)
	if err != nil {
		log.Printf("Failed to generate summary: %v", err)
		// On error, proceed with original messages
		return &ProcessedPrompt{
			Messages:        replacedSystemMsgs,
			IsCompressed:    false,
			SemanticLatency: semanticLatency,
			SemanticContext: semanticContext,
			SummaryLatency:  0,
		}, nil
	}

	// Build final messages
	recentMessages := utils.GetRecentUserMsgsWithNum(req.Messages, p.config.RecentUserMsgUsedNums)
	finalMessages := p.assmebleSummaryMessages(utils.GetSystemMsg(replacedSystemMsgs), summary, recentMessages)
	summaryLatency = time.Since(summaryStart).Milliseconds()

	return &ProcessedPrompt{
		Messages:        finalMessages,
		IsCompressed:    true,
		SemanticLatency: semanticLatency,
		SemanticContext: semanticContext,
		SummaryLatency:  summaryLatency,
	}, nil
}

// BuildUserSummaryMessages builds the final messages with user prompt summary
func (p *CompressionProcessor) assmebleSummaryMessages(systemMsg types.Message, summary string, recentMessages []types.Message) []types.Message {
	log.Printf("[assmebleSummaryMessages] start assmeble summary messages")
	var finalMessages []types.Message

	// Add system message
	finalMessages = append(finalMessages, systemMsg)

	// Add summary as context
	finalMessages = append(finalMessages, types.Message{
		Role:    "assistant",
		Content: summary,
	})

	// Add system message
	finalMessages = append(finalMessages, recentMessages...)

	return finalMessages
}

// trimMessagesToTokenThreshold checks and removes messages from the front until token count is below threshold
func (p *CompressionProcessor) trimMessagesToTokenThreshold(semanticContext string, messagesToSummarize []types.Message) []types.Message {
	// Calculate total tokens
	semanticContextTokens := p.tokenCounter.CountTokens(semanticContext)
	messagesTokens := p.tokenCounter.CountMessagesTokens(messagesToSummarize)
	totalTokens := semanticContextTokens + messagesTokens + 5000

	// Remove messages from front if exceeding threshold
	removedCount := 0
	for totalTokens > p.config.SummaryModelTokenThreshold && len(messagesToSummarize) > 0 {
		removedTokens := p.tokenCounter.CountOneMesaageTokens(messagesToSummarize[0])
		totalTokens -= removedTokens
		messagesToSummarize = messagesToSummarize[1:]
		removedCount++
	}

	log.Printf("[trimMessagesToTokenThreshold] totalTokens: %d, removedCount: %d\n", totalTokens, removedCount)
	if removedCount > 0 {
		log.Printf("[trimMessagesToTokenThreshold] Removed %d messages to meet token threshold", removedCount)
	}

	return messagesToSummarize
}
