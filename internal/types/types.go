package types

type ChatCompletionRequest struct {
	Model         string        `json:"model"`
	Messages      []Message     `json:"messages"`
	Stream        bool          `json:"stream,optional"`
	Temperature   float64       `json:"temperature,optional"`
	StreamOptions StreamOptions `json:"stream_options,optional"`
	ExtraBody     ExtraBody     `json:"extra_body,optional"`
}

type ExtraBody struct {
	PromptMode PromptMode `json:"prompt_mode,optional"`
}

type ChatCompletionResponse struct {
	Id      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage,optional"`
}

type ChatLLMRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,optional"`
}

type ChatLLMRequestStream struct {
	Model         string        `json:"model"`
	Messages      []Message     `json:"messages"`
	Stream        bool          `json:"stream,optional"`
	Temperature   float64       `json:"temperature,optional"`
	StreamOptions StreamOptions `json:"stream_options,optional"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message,optional"`
	Delta        Message `json:"delta,optional"`
	FinishReason string  `json:"finish_reason,optional"`
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

type Identity struct {
	TaskID      string `json:"task_id"`
	RequestID   string `json:"request_id"`
	ClientID    string `json:"client_id"`
	UserName    string `json:"user_name"`
	ProjectPath string `json:"project_path"`
	AuthToken   string `json:"auth_token"`
	LoginFrom   string `json:"login_from"`
}
