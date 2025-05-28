package logic

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

func TestChatCompletionLogic_NewChatCompletionLogic(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			MainModelEndpoint: "http://localhost:8080",
		},
	}

	logic := NewChatCompletionLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
}

func TestChatCompletionLogic_getPromptSample(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	logic := NewChatCompletionLogic(ctx, svcCtx)

	// 测试空消息
	messages := []types.Message{}
	sample := logic.getPromptSample(messages)
	assert.Equal(t, "", sample)

	// 测试只有用户消息
	messages = []types.Message{
		{Role: "user", Content: "Hello, world!"},
	}
	sample = logic.getPromptSample(messages)
	assert.Equal(t, "Hello, world!", sample)

	// 测试混合消息，应该返回最后一个用户消息
	messages = []types.Message{
		{Role: "user", Content: "First message"},
		{Role: "assistant", Content: "Assistant response"},
		{Role: "user", Content: "Second message"},
	}
	sample = logic.getPromptSample(messages)
	assert.Equal(t, "Second message", sample)

	// 测试没有用户消息，应该返回最后一条消息
	messages = []types.Message{
		{Role: "system", Content: "System message"},
		{Role: "assistant", Content: "Assistant message"},
	}
	sample = logic.getPromptSample(messages)
	assert.Equal(t, "Assistant message", sample)
}

func TestChatCompletionLogic_buildPromptFromMessages(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	logic := NewChatCompletionLogic(ctx, svcCtx)

	messages := []types.Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	prompt := logic.buildPromptFromMessages(messages)
	expected := "system: You are a helpful assistant\nuser: Hello\nassistant: Hi there!\n"
	assert.Equal(t, expected, prompt)
}

func TestChatCompletionLogic_countTokensInMessages_Fallback(t *testing.T) {
	ctx := context.Background()

	// 不设置 TokenCounter，测试回退逻辑
	svcCtx := &svc.ServiceContext{
		TokenCounter: nil,
	}

	logic := NewChatCompletionLogic(ctx, svcCtx)

	messages := []types.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	count := logic.countTokensInMessages(messages)
	assert.Greater(t, count, 0) // 应该返回估算的token数
}

func TestChatCompletionLogic_countTokens_Fallback(t *testing.T) {
	ctx := context.Background()

	// 不设置 TokenCounter，测试回退逻辑
	svcCtx := &svc.ServiceContext{
		TokenCounter: nil,
	}

	logic := NewChatCompletionLogic(ctx, svcCtx)

	text := "Hello, world!"
	count := logic.countTokens(text)
	assert.Greater(t, count, 0) // 应该返回估算的token数
}

func TestChatCompletionLogic_ChatCompletion_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			MainModelEndpoint: "", // 空的端点应该导致错误
		},
	}

	logic := NewChatCompletionLogic(ctx, svcCtx)

	// 测试空的消息列表
	req := &types.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []types.Message{},
		Stream:   false,
	}

	resp, err := logic.ChatCompletion(req, make(http.Header))
	// 由于没有有效的端点，应该会出错
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestChatCompletionLogic_ChatCompletion_BasicRequest(t *testing.T) {
	ctx := context.Background()

	var c config.Config
	conf.MustLoad("../../etc/chat-api.yaml", &c)
	svcCtx := svc.NewServiceContext(c)
	defer svcCtx.Stop()

	logic := NewChatCompletionLogic(ctx, svcCtx)

	req := &types.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []types.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
		Temperature: 0.7,
	}

	// 测试基本请求
	resp, err := logic.ChatCompletion(req, make(http.Header))

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
	ctx := context.Background()

	var c config.Config
	conf.MustLoad("../../etc/chat-api.yaml", &c)
	svcCtx := svc.NewServiceContext(c)
	defer svcCtx.Stop()

	logic := NewChatCompletionLogic(ctx, svcCtx)

	req := &types.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []types.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
		Stream:      true, // 流式请求
		Temperature: 0.7,
		ClientId:    "test-client",
		ProjectPath: "/test/path",
		StreamOptions: types.StreamOptions{
			IncludeUsage: true,
		},
	}

	// 测试流式请求
	mockWriter := &mockResponseWriter{}
	err := logic.ChatCompletionStream(req, mockWriter, make(http.Header))

	// 验证响应
	assert.NoError(t, err)
	// 验证确实收到了SSE数据
	assert.Greater(t, len(mockWriter.data), 0)
}
