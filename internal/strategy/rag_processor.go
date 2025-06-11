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

// RagProcessor processes prompts with RAG compression
type RagProcessor struct {
	ctx              context.Context
	semanticClient   *client.SemanticClient
	summaryProcessor *SummaryProcessor
	tokenCounter     *utils.TokenCounter
	config           config.Config
	identity         *types.Identity
}

// NewRagProcessor creates a new compression processor
func NewRagProcessor(ctx context.Context, svcCtx *svc.ServiceContext, identity *types.Identity) (*RagProcessor, error) {
	llmClient, err := client.NewLLMClient(svcCtx.Config.LLMEndpoint, svcCtx.Config.SummaryModel, svcCtx.ReqCtx.Headers)
	if err != nil {
		return nil, err
	}

	return &RagProcessor{
		ctx:              ctx,
		semanticClient:   client.NewSemanticClient(svcCtx.Config.SemanticApiEndpoint),
		summaryProcessor: NewSummaryProcessor(svcCtx.Config.SystemPromptSplitter, llmClient),
		config:           svcCtx.Config,
		tokenCounter:     svcCtx.TokenCounter,
		identity:         identity,
	}, nil
}

// searchSemanticContext performs semantic search and constructs context string
func (p *RagProcessor) searchSemanticContext(ctx context.Context, query string) (string, error) {
	// Prepare semantic request
	semanticReq := client.SemanticRequest{
		ClientId:    p.identity.ClientID,
		ProjectPath: p.identity.ProjectPath,
		Query:       query,
		TopK:        p.config.TopK,
	}

	// Execute search
	semanticResp, err := p.semanticClient.Search(ctx, semanticReq)
	if err != nil {
		err := fmt.Errorf("failed to search semantic:\n%w", err)
		log.Printf("[buildSemanticContext] error: %v", err)
		return "", err
	}

	// Build context string from results
	var contextParts []string
	log.Printf("[buildSemanticContext] Semantic search results nums: %v", len(semanticResp.Results))
	for _, result := range semanticResp.Results {
		if result.Score < p.config.SemanticSocreThreshold {
			continue
		}

		contextParts = append(contextParts, fmt.Sprintf("File: %s (Line %d)\n%s",
			result.FilePath, result.LineNumber, result.Content))
	}

	semanticContext := strings.Join(contextParts, "\n\n")
	log.Printf("[buildSemanticContext] Searched semantic context: %s", semanticContext)

	return semanticContext, nil
}

// replaceSysMsgWithCompressed replaces system messages in the message list
// messages: original message list
func (p *RagProcessor) replaceSysMsgWithCompressed(messages []types.Message) []types.Message {
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

	// If no system message is found, return the original message list
	if !hasSystem {
		return messages
	}
	return processedMsgs
}

// Process process the prompt with RAG compression
func (p *RagProcessor) Process(messages []types.Message) (*ProcessedPrompt, error) {
	var semanticLatency, summaryLatency int64
	prcceedPrompt := &ProcessedPrompt{
		Messages: messages,
	}

	// Get the latest user message
	latestUserMessage, err := utils.GetLatestUserMsg(messages)
	if err != nil {
		return prcceedPrompt, fmt.Errorf("no user message found: %w", err)
	}

	// Record start time for semantic search
	semanticStart := time.Now()
	semanticContext, err := p.searchSemanticContext(p.ctx, latestUserMessage)
	if err != nil {
		log.Printf("Failed to search semantic: %v\n", err)
		prcceedPrompt.SemanticErr = err
	}

	semanticLatency = time.Since(semanticStart).Milliseconds()
	prcceedPrompt.SemanticLatency = semanticLatency
	prcceedPrompt.SemanticContext = semanticContext

	// Replace system messages with compressed messages
	replacedSystemMsgs := p.replaceSysMsgWithCompressed(messages)

	userMessageTokens := p.tokenCounter.CountMessagesTokens(utils.GetUserMsgs(messages))
	needsCompressUserMsg := userMessageTokens > p.config.TokenThreshold
	log.Printf("[process] userMessageTokens: %v, needsCompression: %v\n\n", userMessageTokens, needsCompressUserMsg)

	if !needsCompressUserMsg {
		log.Printf("[process] No need to compress user message\n")
		prcceedPrompt.Messages = replacedSystemMsgs
		return prcceedPrompt, nil
	}

	log.Printf("[process] start compress user prompt message\n")
	// Record start time for summary process
	summaryStart := time.Now()
	// Get messages to summarize (exclude system messages and num-th user message)
	messagesToSummarize := utils.GetOldUserMsgsWithNum(messages, p.config.RecentUserMsgUsedNums)
	messagesToSummarize = p.trimMessagesToTokenThreshold(semanticContext, messagesToSummarize)

	summary, err := p.summaryProcessor.GenerateUserPromptSummary(p.ctx, semanticContext, messagesToSummarize)
	if err != nil {
		log.Printf("[process] Failed to generate summary: %v\n", err)
		// On error, proceed with original messages
		prcceedPrompt.SummaryErr = err
		prcceedPrompt.Messages = replacedSystemMsgs
		return prcceedPrompt, nil
	}

	// Sumary successfuly, build final messages
	recentMessages := utils.GetRecentUserMsgsWithNum(messages, p.config.RecentUserMsgUsedNums)
	finalMessages := p.assmebleSummaryMessages(utils.GetSystemMsg(replacedSystemMsgs), summary, recentMessages)
	summaryLatency = time.Since(summaryStart).Milliseconds()

	prcceedPrompt.SummaryLatency = summaryLatency
	prcceedPrompt.Messages = finalMessages
	prcceedPrompt.IsCompressed = true
	return prcceedPrompt, nil
}

// BuildUserSummaryMessages builds the final messages with user prompt summary
func (p *RagProcessor) assmebleSummaryMessages(systemMsg types.Message, summary string, recentMessages []types.Message) []types.Message {
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
func (p *RagProcessor) trimMessagesToTokenThreshold(semanticContext string, messagesToSummarize []types.Message) []types.Message {
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

	log.Printf(
		"[trimMessagesToTokenThreshold] totalTokens: %d, removedCount: %d, usedCount: %d\n",
		totalTokens, removedCount, len(messagesToSummarize),
	)
	if removedCount > 0 {
		log.Printf("[trimMessagesToTokenThreshold] Removed %d messages to meet token threshold", removedCount)
	}

	return messagesToSummarize
}
