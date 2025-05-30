package strategy

import (
	"context"
	"fmt"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// SUMMARY_PROMPT defines the template for conversation summarization
const SUMMARY_PROMPT = `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.
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

// SummaryProcessor handles conversation summarization
type SummaryProcessor struct {
	llmClient *client.LLMClient
}

// NewSummaryProcessor creates a new summary processor
func NewSummaryProcessor(llmClient *client.LLMClient) *SummaryProcessor {
	return &SummaryProcessor{
		llmClient: llmClient,
	}
}

// GenerateSummary generates a summary of the conversation
func (p *SummaryProcessor) GenerateSummary(ctx context.Context, semanticContext string, messages []types.Message) (string, error) {
	// Create a new slice of messages for the summary request
	var summaryMessages []types.Message

	// Add system message with the summary prompt
	summaryMessages = append(summaryMessages, types.Message{
		Role:    "system",
		Content: SUMMARY_PROMPT,
	})

	for _, msg := range messages {
		if msg.Role != "system" {
			summaryMessages = append(summaryMessages, msg)
		}
	}

	// Add final user instruction
	summaryMessages = append(summaryMessages, types.Message{
		Role:    "user",
		Content: "Summarize the conversation so far, as described in the prompt instructions.",
	})

	// Generate summary using LLM with the new message format
	fmt.Println("==> [GenerateSummary] GenerateSummary messages:", summaryMessages)
	summary, err := p.llmClient.GenerateContent(ctx, summaryMessages)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	fmt.Println("==> [GenerateSummary] summary:", summary)

	return summary, nil
}

// BuildSummaryMessages builds the final messages with summary
func (p *SummaryProcessor) BuildSummaryMessages(messages []types.Message, summary string) []types.Message {
	var finalMessages []types.Message

	// Add system message if exists
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
