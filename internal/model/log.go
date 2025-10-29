package model

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/types"
)

// LatencyMetrics represents latency metrics in milliseconds
type LatencyMetrics struct {
	MainModelLatency  int64 `json:"main_model_latency_ms"`
	TotalLatency      int64 `json:"total_latency_ms"`
	FirstTokenLatency int64 `json:"first_token_latency_ms"`
	WindowLatency     int64 `json:"window_latency_ms"`
}

type ToolCall struct {
	ToolName     string `json:"tool_name"`
	ToolInput    string `json:"tool_input"`
	ToolOutput   string `json:"tool_output"`
	ResultStatus string `json:"result_status"`
	Latency      int64  `json:"latency"`
	Error        string `json:"error"`
}

// RequestParams represents the request parameters for a chat completion
type RequestParams struct {
	Model               string   `json:"model"`
	PromptMode          string   `json:"prompt_mode"`
	MaxTokens           *int     `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int     `json:"max_completion_tokens,omitempty"`
	Temperature         *float64 `json:"temperature,omitempty"`
}

// ChatLog represents a single chat completion log entry
type ChatLog struct {
	Identity  Identity      `json:"identity"`
	Timestamp time.Time     `json:"timestamp"`
	Params    RequestParams `json:"params"`
	// Agent information
	Agent string `json:"agent,omitempty"`
	// Token statistics
	Tokens types.TokenMetrics `json:"tokens"`

	// Processing flags
	IsPromptProceed bool `json:"is_prompt_proceed"`

	// Latency metrics
	Latency LatencyMetrics `json:"latency"`

	// Tools
	ToolCalls []ToolCall `json:"tool_calls"`

	// Content samples (truncated for logging)
	OriginalPrompt  []types.Message `json:"original_prompt"`
	ProcessedPrompt []types.Message `json:"processed_prompt"`

	// Response information
	ResponseHeaders []map[string]string `json:"response_headers,omitempty"`
	ResponseContent string              `json:"response_content,omitempty"`
	Usage           types.Usage         `json:"usage,omitempty"`

	// Classification (will be filled by async processor)
	Category string `json:"category,omitempty"`

	// Error information
	Error []map[types.ErrorType]string `json:"error,omitempty"`
}

// toStringJSON converts the log entry to indented JSON string
func (cl *ChatLog) toStringJSON(indent string) (string, error) {
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)
	err := encoder.Encode(cl)
	if err != nil {
		return "", err
	}
	// Remove the newline added by Encode()
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// ToCompressedJSON converts the log entry to JSON string
func (cl *ChatLog) ToCompressedJSON() (string, error) {
	return cl.toStringJSON("")
}

// ToPrettyJSON Using 2 spaces for compact yet readable indentation (standard JSON formatting practice)
func (cl *ChatLog) ToPrettyJSON() (string, error) {
	return cl.toStringJSON("  ")
}

// FromJSON creates a ChatLog from JSON string
func FromJSON(jsonStr string) (*ChatLog, error) {
	var log ChatLog
	err := json.Unmarshal([]byte(jsonStr), &log)
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// Helper functions

// AddError adds an error entry with type and message to the ChatLog
func (cl *ChatLog) AddError(errorType types.ErrorType, err error) {
	if cl.Error == nil {
		cl.Error = make([]map[types.ErrorType]string, 0)
	}
	cl.Error = append(cl.Error, map[types.ErrorType]string{
		errorType: err.Error(),
	})
}
