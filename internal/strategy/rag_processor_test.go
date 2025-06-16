package strategy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/client/mocks"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

func TestNewRagProcessor(t *testing.T) {

	// Setup mock dependencies
	ctx := context.Background()
	tokenCounter, err := utils.NewTokenCounter()
	assert.Nil(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			LLMEndpoint:            "http://llm.example.com",
			SemanticApiEndpoint:    "http://semantic.example.com",
			SummaryModel:           "gpt-4",
			SystemPromptSplitter:   "==========",
			TopK:                   5,
			SemanticScoreThreshold: 0.5,
			TokenThreshold:         1000,
		},
		TokenCounter: tokenCounter,
		ReqCtx: &svc.RequestContext{
			Headers: &http.Header{},
		},
	}
	identity := &types.Identity{ClientID: "test-client", ProjectPath: "/test/path"}

	// Test successful creation
	processor, err := NewRagProcessor(ctx, svcCtx, identity)
	assert.Nil(t, err)
	assert.NotNil(t, processor)
	assert.Equal(t, ctx, processor.ctx)
	assert.Equal(t, svcCtx.Config, processor.config)
	assert.Equal(t, identity, processor.identity)
}

func TestSearchSemanticContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSemantic := mocks.NewMockSemanticInterface(ctrl)

	processor := &RagProcessor{
		ctx:            context.Background(),
		semanticClient: mockSemantic,
		config: config.Config{
			TopK:                   5,
			SemanticScoreThreshold: 0.5,
		},
		identity: &types.Identity{ClientID: "test-client", ProjectPath: "/test/path"},
	}

	// Test successful search
	t.Run("Success", func(t *testing.T) {
		query := "test query"
		expectedReq := client.SemanticRequest{
			ClientId:    processor.identity.ClientID,
			ProjectPath: processor.identity.ProjectPath,
			Query:       query,
			TopK:        processor.config.TopK,
		}
		mockSemantic.EXPECT().
			Search(gomock.Any(), expectedReq).
			Return(&client.SemanticResponse{
				Results: []client.SemanticResult{
					{Score: 0.8, FilePath: "file1", LineNumber: 10, Content: "content1"},
					{Score: 0.6, FilePath: "file2", LineNumber: 20, Content: "content2"},
					{Score: 0.4, FilePath: "file3", LineNumber: 30, Content: "content3"}, // should be filtered
				},
			}, nil)

		result, err := processor.searchSemanticContext(processor.ctx, query)
		assert.Nil(t, err)
		assert.Contains(t, result, "file1")
		assert.Contains(t, result, "file2")
		assert.NotContains(t, result, "file3")
	})

	// Test search error
	t.Run("Error", func(t *testing.T) {
		query := "error query"
		mockSemantic.EXPECT().
			Search(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("search failed"))

		_, err := processor.searchSemanticContext(processor.ctx, query)
		assert.NotNil(t, err)
	})
}

func TestReplaceSysMsgWithCompressed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLM := mocks.NewMockLLMClientInterface(ctrl)

	processor := &RagProcessor{
		ctx: context.Background(),
		summaryProcessor: &SummaryProcessor{
			llmClient: mockLLM,
		},
	}

	// Test with system message
	t.Run("WithSystemMessage", func(t *testing.T) {
		messages := []types.Message{
			{Role: "system", Content: "system message"},
			{Role: "user", Content: "user message 1"},
		}

		mockLLM.EXPECT().
			GenerateContent(gomock.Any(), gomock.Any(), gomock.Any()).
			Return("compressed system message", nil).
			AnyTimes()

		result := processor.replaceSysMsgWithCompressed(messages)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, "system message", result[0].Content)
	})

	// Test without system message
	t.Run("WithoutSystemMessage", func(t *testing.T) {
		messages := []types.Message{
			{Role: "user", Content: "user message 1"},
		}

		result := processor.replaceSysMsgWithCompressed(messages)
		assert.Equal(t, messages, result)
	})
}

func TestTrimMessagesToTokenThreshold(t *testing.T) {
	tokenCounter, err := utils.NewTokenCounter()
	assert.Nil(t, err)

	processor := &RagProcessor{
		ctx:          context.Background(),
		tokenCounter: tokenCounter,
		config: config.Config{
			SummaryModelTokenThreshold: 1000,
		},
	}

	// Create test messages
	semanticContext := strings.Repeat("test ", 100) // ~100 tokens
	messages := make([]types.Message, 10)
	for i := range messages {
		messages[i] = types.Message{
			Role:    "user",
			Content: fmt.Sprintf("message %d ", i) + strings.Repeat("content ", 20), // ~20 tokens per message
		}
	}

	// Test trimming
	result := processor.trimMessagesToTokenThreshold(semanticContext, messages)
	assert.True(t, len(result) < len(messages))
}

func TestProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mock dependencies
	mockLLM := mocks.NewMockLLMClientInterface(ctrl)
	mockSemantic := mocks.NewMockSemanticInterface(ctrl)

	tokenCounter, err := utils.NewTokenCounter()
	assert.Nil(t, err)

	// Initialize identity for processor
	identity := &types.Identity{ClientID: "test-client", ProjectPath: "/test/path"}

	processor := &RagProcessor{
		ctx:            context.Background(),
		semanticClient: mockSemantic,
		summaryProcessor: &SummaryProcessor{
			systemPromptSplitter: "==========",
			llmClient:            mockLLM,
		},
		tokenCounter: tokenCounter,
		config: config.Config{
			TopK:                       5,
			SemanticScoreThreshold:     0.5,
			TokenThreshold:             500,
			RecentUserMsgUsedNums:      2,
			SummaryModelTokenThreshold: 2000,
			SystemPromptSplitter:       "==========", // 添加splitter配置
		},
		identity: identity,
	}

	// Test normal process flow
	t.Run("NormalFlow", func(t *testing.T) {
		messages := []types.Message{
			{Role: "system", Content: "system message"},
			{Role: "user", Content: "user message 1"},
			{Role: "user", Content: "user message 2"},
		}

		// Setup mock expectations
		mockLLM.EXPECT().GetModelName().Return("test-model").AnyTimes()

		// Mock semantic search
		expectedReq := client.SemanticRequest{
			ClientId:    processor.identity.ClientID,
			ProjectPath: processor.identity.ProjectPath,
			Query:       "user message 2",
			TopK:        processor.config.TopK,
		}
		mockSemantic.EXPECT().
			Search(gomock.Any(), expectedReq).
			Return(&client.SemanticResponse{
				Results: []client.SemanticResult{
					{Score: 0.8, FilePath: "file1", LineNumber: 10, Content: "context1"},
				},
			}, nil).Times(1)

		// Mock summary generation - now handled by GenerateUserPromptSummary
		mockLLM.EXPECT().
			GenerateContent(gomock.Any(), USER_SUMMARY_PROMPT, gomock.Any()).
			Return("summary content", nil).
			Times(1)

		result, err := processor.Process(messages)
		assert.Nil(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result)
		if result.IsCompressed {
			assert.GreaterOrEqual(t, result.SemanticLatency, int64(0))
			assert.GreaterOrEqual(t, result.SummaryLatency, int64(0))
		}
	})

	// Test semantic search failure
	t.Run("SemanticSearchError", func(t *testing.T) {
		messages := []types.Message{
			{Role: "system", Content: "system message"},
			{Role: "user", Content: "query with error"},
		}

		mockSemantic.EXPECT().
			Search(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("search failed"))

		result, err := processor.Process(messages)
		assert.Nil(t, err)
		assert.NotNil(t, result.SemanticErr)
	})

	// Test summary generation failure
	t.Run("SummaryError", func(t *testing.T) {
		// Create messages that will exceed token threshold
		longMessages := make([]types.Message, 20)
		for i := range longMessages {
			longMessages[i] = types.Message{
				Role:    "user",
				Content: fmt.Sprintf("long message %d %s", i, strings.Repeat("content ", 100)),
			}
		}

		// Mock semantic search
		mockSemantic.EXPECT().
			Search(gomock.Any(), gomock.Any()).
			Return(&client.SemanticResponse{
				Results: []client.SemanticResult{
					{Score: 0.8, FilePath: "file1", LineNumber: 10, Content: "context1"},
				},
			}, nil)

		// Mock failed summary
		mockLLM.EXPECT().
			GenerateContent(
				gomock.Any(), // ctx
				gomock.Any(), // prompt
				gomock.Any(), // messages
			).
			Return("", errors.New("summary failed")).
			AnyTimes()

		result, err := processor.Process(longMessages)
		assert.Nil(t, err)
		assert.NotNil(t, result.Messages)
	})
}
