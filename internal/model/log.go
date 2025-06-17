package model

import (
	"bytes"
	"encoding/json"
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

// ChatLog represents a single chat completion log entry
type ChatLog struct {
	Identity  types.Identity `json:"identity"`
	Timestamp time.Time      `json:"timestamp"`
	Model     string         `json:"model"`

	// Token statistics
	OriginalTokens   TokenStats `json:"original_tokens"`
	CompressedTokens TokenStats `json:"compressed_tokens"`
	CompressionRatio float64    `json:"compression_ratio"`

	// Processing flags
	IsPromptProceed        bool `json:"is_prompt_proceed"`
	IsUserPromptCompressed bool `json:"is_user_prompt_compressed"`

	// Latency metrics (in milliseconds)
	SemanticLatency  int64 `json:"semantic_latency_ms"`
	SummaryLatency   int64 `json:"summary_latency_ms"`
	MainModelLatency int64 `json:"main_model_latency_ms"`
	TotalLatency     int64 `json:"total_latency_ms"`

	// Semantic context
	SemanticContext string `json:"semantic_context"`

	// Content samples (truncated for logging)
	OriginalPrompt   []types.Message `json:"original_prompt"`
	CompressedPrompt []types.Message `json:"compressed_prompt"`

	// Response information
	ResponseContent string      `json:"response_content,omitempty"`
	Usage           types.Usage `json:"usage,omitempty"`

	// Classification (will be filled by async processor)
	Category string `json:"category,omitempty"`

	// Error information
	Error []map[types.ErrorType]string `json:"error,omitempty"`
}

// ToJSON converts the log entry to JSON string
func (cl *ChatLog) ToJSON() (string, error) {
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(cl)
	if err != nil {
		return "", err
	}
	// Remove the newline added by Encode()
	return strings.TrimSuffix(buf.String(), "\n"), nil
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
	logCopy.CompressedPrompt = nil
	logJSON, _ := logCopy.ToJSON()

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
