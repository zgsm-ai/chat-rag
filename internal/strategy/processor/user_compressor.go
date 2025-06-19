package processor

import (
	"context"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

// USER_SUMMARY_PROMPT defines the template for conversation user prompt summarization
const USER_SUMMARY_PROMPT = `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.
This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be essential for continuing with the conversation and supporting any continuing tasks.

Your summary should be structured as follows:
Context: The context to continue the conversation with. If applicable based on the current task, this should include:
  1. Previous Conversation: High level details about what was discussed throughout the entire conversation with the user. This should be written to allow someone to be able to follow the general overarching conversation flow.
  2. Current Work: Describe in detail what was being worked on prior to this request to summarize the conversation. Pay special attention to the more recent messages in the conversation.
  3. Key Technical Concepts: List all important technical concepts, technologies, coding conventions, and frameworks discussed, which might be relevant for continuing with this work.
  4. Relevant Files and Code: If applicable, enumerate specific files and code sections examined, modified, or created for the task continuation. Pay special attention to the most recent messages and changes.
  5. Problem Solving: Document problems solved thus far and any ongoing troubleshooting efforts.
  6. Pending Tasks and Next Steps: Outline all pending tasks that you have explicitly been asked to work on, as well as list the next steps you will take for all outstanding work, if applicable. Include code snippets where they add clarity. For any next steps, include direct quotes from the most recent conversation showing exactly what task you were working on and where you left off. This should be verbatim to ensure there's no information loss in context between tasks.
  7. Language: Emphasize the language mentioned by system.

Example summary structure:
1. Previous Conversation:
  [Detailed description]
2. Current Work:
  [Detailed description]
3. Key Technical Concepts:
  - [Concept 1]
  - [Concept 2]
  - [...]
4. Relevant Files and Code:
  - [File Name 1]
    - [Summary of why this file is important]
    - [Summary of the changes made to this file, if any]
    - [Important Code Snippet]
  - [File Name 2]
    - [Important Code Snippet]
  - [...]
5. Problem Solving:
  [Detailed description]
6. Pending Tasks and Next Steps:
  - [Task 1 details & next steps]
  - [Task 2 details & next steps]
  - [...]
7. Langeuage:
	[Always answer in the language]

Output only the summary of the conversation so far, without any additional commentary or explanation.`

type UserCompressor struct {
	Recorder
	ctx          context.Context
	config       config.Config
	llmClient    client.LLMInterface
	tokenCounter *tokenizer.TokenCounter
	next         Processor
}

func NewUserCompressor(
	ctx context.Context,
	config config.Config,
	llmClient client.LLMInterface,
	tokenCounter *tokenizer.TokenCounter,
) *UserCompressor {
	return &UserCompressor{
		ctx:          ctx,
		config:       config,
		llmClient:    llmClient,
		tokenCounter: tokenCounter,
	}
}

func (u *UserCompressor) Execute(promptMsg *PromptMsg) {
	logger.Info("[SystemCompressor] starting system prompt compression",
		zap.String("method", "Execute"),
	)
	if promptMsg == nil {
		logger.Error("promptMsg is nil!")
		return
	}

	logger.Info("start compress user prompt message",
		zap.String("method", "Process"),
	)
	// Record start time for summary process
	summaryStart := time.Now()
	// Get messages to summarize (exclude system messages and num-th user message)
	messagesToSummarize := utils.GetOldUserMsgsWithNum(replacedSystemMsgs, p.config.RecentUserMsgUsedNums)
	messagesToSummarize = s.trimMessagesToTokenThreshold(semanticContext, messagesToSummarize)

	summary, err := s.generateUserPromptSummary(s.ctx, semanticContext, messagesToSummarize)
	if err != nil {
		logger.Error("failed to generate summary",
			zap.Error(err),
			zap.String("method", "Process"),
		)
		// On error, proceed with original messages
		proceedPrompt.SummaryErr = err
		proceedPrompt.Messages = replacedSystemMsgs
		return proceedPrompt, nil
	}

	if s.next != nil {
		s.next.Execute(promptMsg)
	} else {
		logger.Error("system prompt compression completed, but no next processor found",
			zap.String("method", "Execute"),
		)
	}
}

func (s *UserCompressor) SetNext(next Processor) {
	s.next = next
}

// generateUserPromptSummary generates a user prompt summary of the conversation
func (u *UserCompressor) generateUserPromptSummary(ctx context.Context, semanticContext string, messages []types.Message) (string, error) {
	logger.Info("start generating user prompt summary",
		zap.String("model", u.llmClient.GetModelName()),
		zap.String("method", "GenerateUserPromptSummary"),
	)
	// Create a new slice of messages for the summary request
	var summaryMessages []types.Message

	for _, msg := range messages {
		if msg.Role != "system" {
			summaryMessages = append(summaryMessages, msg)
		}
	}

	if semanticContext != "" {
		summaryMessages = append(summaryMessages, types.Message{
			Role:    "assistant",
			Content: "semanticContext: " + semanticContext + "\n\n",
		})
	}

	// Add final user instruction
	summaryMessages = append(summaryMessages, types.Message{
		Role:    "user",
		Content: "Summarize the conversation so far, as described in the prompt instructions.",
	})

	return u.llmClient.GenerateContent(ctx, USER_SUMMARY_PROMPT, summaryMessages)
}

// trimMessagesToTokenThreshold checks and removes messages from the front until token count is below threshold
func (u *UserCompressor) trimMessagesToTokenThreshold(messagesToSummarize []types.Message) []types.Message {
	// Calculate total tokens
	semanticContextTokens := u.tokenCounter.CountTokens(semanticContext)
	messagesTokens := u.tokenCounter.CountMessagesTokens(messagesToSummarize)
	totalTokens := semanticContextTokens + messagesTokens + 5000

	// Remove messages from front if exceeding threshold
	removedCount := 0
	for totalTokens > p.config.SummaryModelTokenThreshold && len(messagesToSummarize) > 0 {
		removedTokens := p.tokenCounter.CountOneMesaageTokens(messagesToSummarize[0])
		totalTokens -= removedTokens
		messagesToSummarize = messagesToSummarize[1:]
		removedCount++
	}

	logger.Info("message token stats",
		zap.Int("totalTokens", totalTokens),
		zap.Int("removedCount", removedCount),
		zap.Int("usedCount", len(messagesToSummarize)),
		zap.String("method", "trimMessagesToTokenThreshold"),
	)
	if removedCount > 0 {
		logger.Info("removed messages to meet threshold",
			zap.Int("count", removedCount),
			zap.String("method", "trimMessagesToTokenThreshold"),
		)
	}

	return messagesToSummarize
}
