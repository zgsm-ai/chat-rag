package model

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/types"
)

// TokenStats represents detailed token statistics
type TokenStats struct {
	SystemTokens int `json:"system_tokens"`
	UserTokens   int `json:"user_tokens"`
	All          int `json:"all"`
}

// TokenRatio represents the ratio between processed and original tokens
type TokenRatio struct {
	SystemRatio float64 `json:"system_ratio"`
	UserRatio   float64 `json:"user_ratio"`
	AllRatio    float64 `json:"all_ratio"`
}

// TokenMetrics represents complete token statistics including original, processed and ratios
type TokenMetrics struct {
	Original  TokenStats `json:"original"`
	Processed TokenStats `json:"processed"`
	Ratios    TokenRatio `json:"ratios"`
}

// CalculateRatios calculates the token ratios between processed and original tokens
func (tm *TokenMetrics) CalculateRatios() {
	if tm.Original.All > 0 {
		ratio := float64(tm.Processed.All) / float64(tm.Original.All)
		tm.Ratios.AllRatio = math.Round(ratio*100) / 100
	}
	if tm.Original.SystemTokens > 0 {
		ratio := float64(tm.Processed.SystemTokens) / float64(tm.Original.SystemTokens)
		tm.Ratios.SystemRatio = math.Round(ratio*100) / 100
	}
	if tm.Original.UserTokens > 0 {
		ratio := float64(tm.Processed.UserTokens) / float64(tm.Original.UserTokens)
		tm.Ratios.UserRatio = math.Round(ratio*100) / 100
	}
}

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

	// Token statistics
	Tokens TokenMetrics `json:"tokens"`

	// Processing flags
	IsPromptProceed        bool `json:"is_prompt_proceed"`
	IsUserPromptCompressed bool `json:"is_user_prompt_compressed"`

	// Latency metrics
	Latency LatencyMetrics `json:"latency"`

	// Tools
	ToolCalls []ToolCall `json:"tool_calls"`

	// Content samples (truncated for logging)
	OriginalPrompt  []types.Message `json:"original_prompt"`
	ProcessedPrompt []types.Message `json:"processed_prompt"`

	// Response information
	ResponseContent string              `json:"response_content,omitempty"`
	ResponseHeaders []map[string]string `json:"response_headers,omitempty"`
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

// LogBatch represents a batch of logs for uploading to Loki
type LogBatch struct {
	Streams []LogStream `json:"streams"`
}

// LogStream represents a stream of logs with labels
type LogStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// LokiLogEntry represents a single log entry for Loki
type LokiLogEntry struct {
	Timestamp string `json:"timestamp"`
	Line      string `json:"line"`
}

// CreateLokiStream creates a Loki-compatible log stream for a single log entry
func CreateLokiStream(log *ChatLog) *LogStream {
	// Create labels for this log entry
	labels := map[string]string{
		"service":    "chat-rag",
		"client_id":  log.Identity.ClientID,
		"compressed": boolToString(log.IsUserPromptCompressed),
	}

	if log.Category != "" {
		labels["category"] = log.Category
	}

	// Add log entry to stream
	logCopy := *log
	logCopy.OriginalPrompt = nil
	logCopy.ProcessedPrompt = nil
	logJSON, _ := logCopy.ToCompressedJSON()

	return &LogStream{
		Stream: labels,
		Values: [][]string{
			{
				timestampToNano(log.Timestamp),
				logJSON,
			},
		},
	}
}

// Helper functions
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func timestampToNano(t time.Time) string {
	return strconv.FormatInt(t.UnixNano(), 10)
}

// AddError adds an error entry with type and message to the ChatLog
func (cl *ChatLog) AddError(errorType types.ErrorType, err error) {
	if cl.Error == nil {
		cl.Error = make([]map[types.ErrorType]string, 0)
	}
	cl.Error = append(cl.Error, map[types.ErrorType]string{
		errorType: err.Error(),
	})
}
