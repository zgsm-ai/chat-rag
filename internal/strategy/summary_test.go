package strategy

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/zgsm-ai/chat-rag/internal/client/mocks"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

func TestGenerateSystemPromptSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLMClient := mocks.NewMockLLMClientInterface(ctrl)
	processor := NewSummaryProcessor("###", mockLLMClient)

	ctx := context.Background()
	testSystemPrompt := "Test system prompt content"
	expectedMessage := types.Message{
		Role:    "user",
		Content: "Please compress the following content:\n" + testSystemPrompt,
	}

	// Mock the expected LLM call
	mockLLMClient.EXPECT().GenerateContent(
		ctx,
		SYSTEM_SUMMARY_PROMPT,
		[]types.Message{expectedMessage},
	).Return("compressed_content", nil)

	// Test the function
	result, err := processor.GenerateSystemPromptSummary(ctx, testSystemPrompt)
	if err != nil {
		t.Errorf("GenerateSystemPromptSummary failed: %v", err)
	}
	if result != "compressed_content" {
		t.Errorf("Expected compressed_content, got %s", result)
	}
}

func TestGenerateUserPromptSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLMClient := mocks.NewMockLLMClientInterface(ctrl)
	processor := NewSummaryProcessor("###", mockLLMClient)

	ctx := context.Background()
	testMessages := []types.Message{
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Message 2"},
	}

	// Mock the expected LLM calls
	mockLLMClient.EXPECT().GetModelName().Return("test-model")
	mockLLMClient.EXPECT().GenerateContent(
		ctx,
		USER_SUMMARY_PROMPT,
		gomock.Any(), // We can't predict exact message slice due to semantic context logic
	).Return("user_summary_content", nil)

	// Test the function
	result, err := processor.GenerateUserPromptSummary(ctx, "test_context", testMessages)
	if err != nil {
		t.Errorf("GenerateUserPromptSummary failed: %v", err)
	}
	if result != "user_summary_content" {
		t.Errorf("Expected user_summary_content, got %s", result)
	}
}

func TestProcessSystemMessageUncached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLMClient := mocks.NewMockLLMClientInterface(ctrl)
	processor := NewSummaryProcessor("###分割符###", mockLLMClient)

	testMsg := types.Message{
		Role:    "system",
		Content: "Pre content###分割符###Content to compress",
	}

	// Clear cache before test
	GetSystemPromptCache().Set(generateHash("Content to compress"), "")

	ctx := context.Background()
	expectedMessage := types.Message{
		Role:    "user",
		Content: "Please compress the following content:\n###分割符###Content to compress",
	}

	exactPrompt := SYSTEM_SUMMARY_PROMPT // 直接使用常量

	// 添加WaitGroup等待异步调用完成
	var wg sync.WaitGroup
	wg.Add(1)

	mockLLMClient.EXPECT().GenerateContent(
		gomock.Eq(ctx),
		gomock.Eq(exactPrompt),
		gomock.Eq([]types.Message{expectedMessage}),
	).Times(1).
		DoAndReturn(func(context.Context, string, []types.Message) (string, error) {
			defer wg.Done()
			return "compressed_content", nil
		})

	result := processor.processSystemMessageWithCache(testMsg)

	// 等待异步调用完成
	wg.Wait()

	if !strings.HasPrefix(result.Content.(string), testMsg.Content.(string)) {
		t.Errorf("Expected processed content when uncached, got %v", result.Content)
	}
}

func TestProcessSystemMessageCached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLMClient := mocks.NewMockLLMClientInterface(ctrl)
	processor := NewSummaryProcessor("###分割符###", mockLLMClient)

	testMsg := types.Message{
		Role:    "system",
		Content: "Pre content###分割符###Content to compress",
	}

	// Pre-populate cache
	GetSystemPromptCache().Set(generateHash("Content to compress"), "cached_compressed")

	result := processor.processSystemMessageWithCache(testMsg)
	if !strings.HasPrefix(result.Content.(string), "Pre content") {
		t.Errorf("Expected cached content with prefix, got %s", result.Content)
	}
}
