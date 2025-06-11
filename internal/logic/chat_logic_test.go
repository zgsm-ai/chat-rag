package logic

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/service"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

// createTestContext 创建测试用的context
func createTestContext() context.Context {
	return context.Background()
}

// createTestServiceContext 创建测试用的ServiceContext
// tokenCounter参数可以是nil或*utils.TokenCounter类型
func createTestServiceContext(cfg *config.Config, tokenCounter interface{}) *svc.ServiceContext {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			LLMEndpoint: cfg.LLMEndpoint,
		},
	}

	// 如果有tokenCounter且类型正确，设置到ServiceContext
	if tc, ok := tokenCounter.(*utils.TokenCounter); ok {
		svcCtx.TokenCounter = tc
	}

	return svcCtx
}

// createTestRequest 创建测试用的ChatCompletionRequest
func createTestRequest(model string, messages []types.Message, stream bool) *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Stream:   stream,
	}
}

// createTestIdentity 创建测试用的Identity
func createTestIdentity() *types.Identity {
	return &types.Identity{
		ClientID:    "test-client",
		ProjectPath: "/test/path",
	}
}

// createTestRequestContext 创建测试用的RequestContext
func createTestRequestContext(req *types.ChatCompletionRequest, writer http.ResponseWriter) *svc.RequestContext {
	headers := make(http.Header)
	return &svc.RequestContext{
		Request: req,
		Writer:  writer,
		Headers: &headers,
	}
}

// setupTestLogic 组合所有辅助函数创建完整的测试逻辑
func setupTestLogic(t *testing.T, cfg *config.Config, tokenCounter interface{},
	model string, messages []types.Message, writer http.ResponseWriter) (*ChatCompletionLogic, *svc.ServiceContext) {
	ctx := createTestContext()
	svcCtx := createTestServiceContext(cfg, tokenCounter)
	req := createTestRequest(model, messages, false)
	reqCtx := createTestRequestContext(req, writer)
	svcCtx.SetRequestContext(reqCtx)
	identity := createTestIdentity()
	return NewChatCompletionLogic(ctx, svcCtx, identity), svcCtx
}

func TestChatCompletionLogic_NewChatCompletionLogic(t *testing.T) {
	mockWriter := &mockResponseWriter{}
	cfg := &config.Config{LLMEndpoint: "http://localhost:8080"}
	logic, svcCtx := setupTestLogic(t, cfg, nil, "test-model", []types.Message{
		{Role: "user", Content: "Hello"},
	}, mockWriter)

	assert.NotNil(t, logic)
	assert.Equal(t, createTestContext(), logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
	// 验证ReqCtx设置是否正确
	assert.NotNil(t, svcCtx.ReqCtx)
	assert.Equal(t, mockWriter, svcCtx.ReqCtx.Writer)
}

// TestLLMClientMock 测试LLMClient mock
func TestLLMClientMock(t *testing.T) {
	mock := &client.LLMClient{}
	assert.NotNil(t, mock)
}

// 测试空消息时的token计算
func TestChatCompletionLogic_countTokens_Empty(t *testing.T) {
	cfg := &config.Config{}
	logic, _ := setupTestLogic(t, cfg, nil, "test-model", []types.Message{}, &mockResponseWriter{})

	count := logic.countTokens("")
	assert.Equal(t, 0, count)

	messages := []types.Message{}
	count = logic.countTokensInMessages(messages)
	assert.Equal(t, 0, count)
}

func TestChatCompletionLogic_countTokensInMessages_Fallback(t *testing.T) {
	cfg := &config.Config{}
	logic, _ := setupTestLogic(t, cfg, nil, "test-model", []types.Message{}, &mockResponseWriter{})

	messages := []types.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	count := logic.countTokensInMessages(messages)
	assert.Greater(t, count, 0) // 应该返回估算的token数
}

func TestChatCompletionLogic_countTokens_Fallback(t *testing.T) {
	cfg := &config.Config{}
	logic, _ := setupTestLogic(t, cfg, nil, "test-model", []types.Message{}, &mockResponseWriter{})

	text := "Hello, world!"
	count := logic.countTokens(text)
	assert.Greater(t, count, 0) // 应该返回估算的token数
}

func TestChatCompletionLogic_ChatCompletion_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.Config
		expected string
	}{
		{
			name:     "empty endpoint",
			config:   &config.Config{LLMEndpoint: ""},
			expected: "NewLLMClient llmEndpoint cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic, svcCtx := setupTestLogic(t, tt.config, nil, "test-model", []types.Message{}, &mockResponseWriter{})
			// 创建mock服务用于测试
			loggerService := service.LoggerService{
				// 使用实际的NewLoggerService函数初始化
			}
			svcCtx.LoggerService = &loggerService
			svcCtx.MetricsService = &service.MetricsService{}

			resp, err := logic.ChatCompletion()
			t.Log("==>", err)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expected)
			assert.Nil(t, resp)
		})
	}

	// 测试有效配置
	t.Run("valid config", func(t *testing.T) {
		cfg := &config.Config{
			LLMEndpoint: "http://test-endpoint",
		}
		_, svcCtx := setupTestLogic(t, cfg, nil, "test-model", []types.Message{}, &mockResponseWriter{})
		assert.NotNil(t, svcCtx)
		assert.Equal(t, "http://test-endpoint", svcCtx.Config.LLMEndpoint)
	})
}

// 测试TokenCounter正确设置的场景
func TestChatCompletionLogic_WithTokenCounter(t *testing.T) {
	mockWriter := &mockResponseWriter{}
	cfg := &config.Config{LLMEndpoint: "http://localhost:8080"}
	tokenCounter := &utils.TokenCounter{}

	logic, svcCtx := setupTestLogic(t, cfg, tokenCounter, "test-model",
		[]types.Message{{Role: "user", Content: "Hello"}}, mockWriter)

	assert.NotNil(t, logic)
	assert.NotNil(t, svcCtx.TokenCounter)
	assert.Equal(t, tokenCounter, svcCtx.TokenCounter)
}

func TestChatCompletionLogic_ChatCompletion_BasicRequest(t *testing.T) {
	cfg := utils.MustLoadConfig("../../etc/chat-api.yaml")
	svcCtx := svc.NewServiceContext(cfg)
	defer svcCtx.Stop()

	logic, _ := setupTestLogic(t, &svcCtx.Config, svcCtx.TokenCounter,
		"gpt-3.5-turbo", []types.Message{
			{Role: "user", Content: "Hello, how are you?"},
		}, &mockResponseWriter{})

	// 测试基本请求
	resp, err := logic.ChatCompletion()

	// 验证响应
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// mockResponseWriter 模拟 http.ResponseWriter 和 http.Flusher 用于测试
type mockResponseWriter struct {
	data       []byte
	headers    http.Header
	statusCode int
	flushed    bool
}

func (m *mockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	m.data = append(m.data, data...)
	return len(data), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

// Flush 实现http.Flusher接口
func (m *mockResponseWriter) Flush() {
	m.flushed = true
}

func TestChatCompletionLogic_ChatCompletion_StreamingRequest(t *testing.T) {
	// 加载配置
	cfg := utils.MustLoadConfig("../../etc/chat-api.yaml")

	// 准备测试数据
	testModel := "gpt-3.5-turbo"
	testMessages := []types.Message{
		{Role: "user", Content: "Hello, how are you?"},
	}
	testWriter := &mockResponseWriter{}

	// 创建服务上下文
	svcCtx := svc.NewServiceContext(cfg)

	// 设置请求上下文
	svcCtx.SetRequestContext(createTestRequestContext(
		createTestRequest(testModel, testMessages, true),
		testWriter,
	))

	// 创建逻辑实例
	logic := NewChatCompletionLogic(
		createTestContext(),
		svcCtx,
		createTestIdentity(),
	)

	// 执行测试
	err := logic.ChatCompletionStream()

	// 验证预期错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401 Authorization Required", "Expected 401 unauthorized error")

	// 验证是否尝试写入响应
	assert.Greater(t, len(testWriter.data), 0, "Expected response attempt data")
	assert.True(t, testWriter.flushed, "Expected response flush attempt")
}
