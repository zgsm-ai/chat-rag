package strategy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strings"
	"sync"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// SYSTEM_SUMMARY_PROMPT defines the template for conversation system prompt summarization
const SYSTEM_SUMMARY_PROMPT = `You are a documentation standardization expert. You will receive a technical specification text. Please compress it to retain key information, operational rules, and core usage principles, while minimizing repetition, verbosity, and secondary descriptions. The goal is to make the content more concise and clear for engineers to quickly understand and implement.

Please strictly follow the requirements below for the compression task:

### Task Objectives:
1. Compress and optimize the technical specification text, extracting key points.
2. Remove redundant or repetitive content and simplify complex sentence structures.
3. Retain key operational rules, tool usage methods, and behavioral constraints.
4. Preserve important restrictions and operational examples completely to avoid missing information.

### Compression Principles:
1. **Information Integrity First**: All necessary usage rules and core constraints must be preserved.
2. **Clear and Concise Expression**: Each sentence should convey a single rule; use bullet points where possible.
3. **Remove Redundant Information**: Eliminate repetitive content, over-explanations, and general knowledge.
4. **Enhance Structural Logic**: Organize content by theme (e.g., tool usage guidelines, editing rules, mode descriptions).

### Output Format:
* Maintain the original Markdown structure and paragraph divisions.
* Divide content into modules using headings (e.g., ## Tool Usage Guidelines).
* Final text length should be 30%-50% of the original to ensure readability, standardization, and structural clarity.
* Output in English only, without additional explanations such as "This is the compressed text.`

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

Output only the summary of the conversation so far, without any additional commentary or explanation.`

// SystemPromptCache is a global singleton cache for system prompt summaries
type SystemPromptCache struct {
	cache map[string]string
	mutex sync.RWMutex
}

var (
	systemPromptCacheInstance *SystemPromptCache
	systemPromptCacheOnce     sync.Once
)

// GetSystemPromptCache returns the singleton instance of SystemPromptCache
func GetSystemPromptCache() *SystemPromptCache {
	systemPromptCacheOnce.Do(func() {
		systemPromptCacheInstance = &SystemPromptCache{
			cache: make(map[string]string),
		}
	})
	return systemPromptCacheInstance
}

// Get retrieves a cached system prompt summary by hash
func (c *SystemPromptCache) Get(hash string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	summary, exists := c.cache[hash]
	return summary, exists
}

// Set stores a system prompt summary in the cache
func (c *SystemPromptCache) Set(hash, summary string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[hash] = summary
}

// generateHash generates a SHA256 hash for the given content
func generateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// SummaryProcessor handles conversation summarization
type SummaryProcessor struct {
	systemPromptSplitter string
	llmClient            *client.LLMClient
}

// NewSummaryProcessor creates a new summary processor
func NewSummaryProcessor(systemPromptSplitter string, llmClient *client.LLMClient) *SummaryProcessor {
	return &SummaryProcessor{
		systemPromptSplitter: systemPromptSplitter,
		llmClient:            llmClient,
	}
}

// GenerateUserPromptSummary generates a user prompt summary of the conversation
func (p *SummaryProcessor) GenerateUserPromptSummary(ctx context.Context, semanticContext string, messages []types.Message) (string, error) {
	log.Println("[GenerateUserPromptSummary] Start generating user prompt summary...")
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

	return p.llmClient.GenerateContent(ctx, USER_SUMMARY_PROMPT, summaryMessages)
}

// GenerateSystemPromptSummary generates a system prompt summary of the conversation
func (p *SummaryProcessor) GenerateSystemPromptSummary(ctx context.Context, systemPrompt string) (string, error) {
	log.Printf("[GenerateSystemPromptSummary] Generating system prompt summary...\n")
	// Create a new slice of messages for the summary request
	var summaryMessages []types.Message

	// Add final user instruction
	summaryMessages = append(summaryMessages, types.Message{
		Role:    "user",
		Content: "Please compress the following content:\n" + systemPrompt,
	})

	return p.llmClient.GenerateContent(ctx, SYSTEM_SUMMARY_PROMPT, summaryMessages)
}

// processSystemMessageWithCache processes system message with caching logic
func (p *SummaryProcessor) processSystemMessageWithCache(msg types.Message) types.Message {
	cache := GetSystemPromptCache()

	// Convert content to string
	systemContent, ok := msg.Content.(string)
	if !ok {
		// If content is not string, use original message
		return msg
	}

	// Check if system prompt contains SystemPromptSplitter
	toolGuidelinesIndex := strings.Index(systemContent, p.systemPromptSplitter)

	// If no SystemPromptSplitter found, use original message without compression
	if toolGuidelinesIndex == -1 {
		log.Printf("[processSystemMessageWithCache] No SystemPromptSplitter found!\n")
		return msg
	}

	// Extract content from SystemPromptSplitter to the end
	contentToCompress := systemContent[toolGuidelinesIndex:]
	contentBeforeGuidelines := systemContent[:toolGuidelinesIndex]

	// Generate hash for the content to be compressed
	systemHash := generateHash(contentToCompress)

	// Check if compressed version exists in cache
	if compressedContent, exists := cache.Get(systemHash); exists {
		log.Printf("[processSystemMessageWithCache] Using cached compressed system prompt\n")
		// Use cached compressed version, combining with content before guidelines
		return types.Message{
			Role:    "system",
			Content: contentBeforeGuidelines + compressedContent,
		}
	} else {
		// Asynchronously compress and cache the guidelines content
		log.Printf("[processSystemMessageWithCache] uncached, generating compressed system prompt for guidelines section\n")
		go func(content, hash string) {
			if compressed, err := p.GenerateSystemPromptSummary(context.Background(), content); err == nil {
				log.Printf("[processSystemMessageWithCache] compressed system prompt")
				cache.Set(hash, compressed)
			}
		}(contentToCompress, systemHash)

		// Use original system prompt
		return msg
	}
}

// BuildUserSummaryMessages builds the final messages with user prompt summary
func (p *SummaryProcessor) BuildUserSummaryMessages(ctx context.Context, messages []types.Message, summary string) []types.Message {
	var finalMessages []types.Message

	// Add system message if exists, with caching logic
	for _, msg := range messages {
		if msg.Role == "system" {
			finalMessages = append(finalMessages, msg)
			break
		}
	}

	// Add summary as context
	finalMessages = append(finalMessages, types.Message{
		Role:    "assistant",
		Content: summary,
	})

	// Add last user message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			finalMessages = append(finalMessages, messages[i])
			break
		}
	}

	return finalMessages
}
