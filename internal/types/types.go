package types

const (
	// RoleSystem System role message
	RoleSystem = "system"

	// RoleUser User role message
	RoleUser = "user"

	// RoleAssistant AI assistant role message
	RoleAssistant = "assistant"
)

// ErrorType defines different types of errors
type ErrorType string

const (
	// ErrSemantic represents semantic processing errors
	ErrSemantic ErrorType = "SemanticError"

	// ErrSummary represents summary generation errors
	ErrSummary ErrorType = "SummaryError"

	// ErrExtra represents extra operation errors
	ErrExtra ErrorType = "ExtraError"
)

// PromptMode defines different types of chat
type PromptMode string

const (
	// Raw mode: No deep processing of user prompt, only necessary operations like compression
	Raw PromptMode = "raw"

	// Balanced mode: Considering both cost and performance, choosing a compromise approach
	// including rag and prompt compression
	Balanced PromptMode = "balanced"

	// Cost Cost-first mode: Minimizing LLM calls and context size to save cost
	Cost PromptMode = "cost"

	// Performance Performance-first mode: Maximizing output quality without considering cost
	Performance PromptMode = "performance"

	// Auto select mode: Default is balanced mode
	Auto PromptMode = "auto"
)

type ChatCompletionRequest struct {
	Model         string        `json:"model"`
	Messages      []Message     `json:"messages"`
	Stream        bool          `json:"stream,omitempty"`
	Temperature   float64       `json:"temperature,omitempty"`
	StreamOptions StreamOptions `json:"stream_options,omitempty"`
	ExtraBody     ExtraBody     `json:"extra_body,omitempty"`
}

type ExtraBody struct {
	PromptMode PromptMode `json:"prompt_mode,omitempty"`
}

type ChatCompletionResponse struct {
	Id      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage,omitempty"`
}

type ChatLLMRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

type ChatLLMRequestStream struct {
	Model         string        `json:"model"`
	Messages      []Message     `json:"messages"`
	Stream        bool          `json:"stream,omitempty"`
	Temperature   float64       `json:"temperature,omitempty"`
	StreamOptions StreamOptions `json:"stream_options,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message,omitempty"`
	Delta        Message `json:"delta,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
