package logic

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zgsm-ai/chat-rag/internal/config"
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
	return NewChatCompletionLogic(ctx, svcCtx), svcCtx
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
	cfg := &config.Config{LLMEndpoint: ""}
	logic, _ := setupTestLogic(t, cfg, nil, "test-model", []types.Message{}, &mockResponseWriter{})

	resp, err := logic.ChatCompletion()
	// 由于没有有效的端点，应该会出错
	assert.Error(t, err)
	assert.Nil(t, resp)
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
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "chat.completion", resp.Object)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "assistant", resp.Choices[0].Message.Role)
	assert.NotEmpty(t, resp.Choices[0].Message.Content)
	assert.Equal(t, "stop", resp.Choices[0].FinishReason)
	assert.NotNil(t, resp.Usage)
	assert.Greater(t, resp.Usage.TotalTokens, 0)
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
	cfg := utils.MustLoadConfig("../../etc/chat-api.yaml")
	svcCtx := svc.NewServiceContext(cfg)
	defer svcCtx.Stop()

	mockWriter := &mockResponseWriter{}
	req := &types.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []types.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
		Stream:      true,
		Temperature: 0.7,
		ClientId:    "test-client",
		ProjectPath: "/test/path",
		StreamOptions: types.StreamOptions{
			IncludeUsage: true,
		},
	}

	reqCtx := createTestRequestContext(req, mockWriter)
	svcCtx.SetRequestContext(reqCtx)
	logic := NewChatCompletionLogic(createTestContext(), svcCtx)

	err := logic.ChatCompletionStream()

	// 验证响应
	assert.NoError(t, err)
	// 验证确实收到了SSE数据
	assert.Greater(t, len(mockWriter.data), 0)
}
