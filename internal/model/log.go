package model

import (
	"encoding/json"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/types"
)

// ChatLog represents a single chat completion log entry
type ChatLog struct {
	RequestID   string    `json:"request_id"`
	Timestamp   time.Time `json:"timestamp"`
	ClientID    string    `json:"client_id"`
	ProjectPath string    `json:"project_path"`
	Model       string    `json:"model"`

	// Token statistics
	OriginalTokens   int     `json:"original_tokens"`
	CompressedTokens int     `json:"compressed_tokens"`
	CompressionRatio float64 `json:"compression_ratio"`

	// Processing flags
	IsCompressed         bool `json:"is_compressed"`
	CompressionTriggered bool `json:"compression_triggered"`

	// Latency metrics (in milliseconds)
	SemanticLatency  int64 `json:"semantic_latency_ms"`
	SummaryLatency   int64 `json:"summary_latency_ms"`
	MainModelLatency int64 `json:"main_model_latency_ms"`
	TotalLatency     int64 `json:"total_latency_ms"`

	// Content samples (truncated for logging)
	OriginalPromptSample   []types.Message `json:"original_prompt_sample"`
	CompressedPromptSample []types.Message `json:"compressed_prompt_sample"`

	// Response information
	ResponseContent string      `json:"response_content,omitempty"`
	Usage           types.Usage `json:"usage,omitempty"`

	// Classification (will be filled by async processor)
	Category string `json:"category,omitempty"`

	// Error information
	Error string `json:"error,omitempty"`
}

// ToJSON converts the log entry to JSON string
func (cl *ChatLog) ToJSON() (string, error) {
	data, err := json.Marshal(cl)
	if err != nil {
		return "", err
	}
	return string(data), nil
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

// TruncateContent truncates content to a specified length for logging
func TruncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "..."
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

// CreateLokiBatch creates a Loki-compatible log batch
func CreateLokiBatch(logs []*ChatLog) *LogBatch {
	streams := make(map[string]*LogStream)

	for _, log := range logs {
		// Create labels for this log entry
		labels := map[string]string{
			"service":    "chat-rag",
			"client_id":  log.ClientID,
			"compressed": boolToString(log.IsCompressed),
		}

		if log.Category != "" {
			labels["category"] = log.Category
		}

		// Create stream key from labels
		streamKey := createStreamKey(labels)

		// Get or create stream
		if streams[streamKey] == nil {
			streams[streamKey] = &LogStream{
				Stream: labels,
				Values: [][]string{},
			}
		}

		// Add log entry to stream
		logJSON, _ := log.ToJSON()
		streams[streamKey].Values = append(streams[streamKey].Values, []string{
			timestampToNano(log.Timestamp),
			logJSON,
		})
	}

	// Convert map to slice
	var streamSlice []LogStream
	for _, stream := range streams {
		streamSlice = append(streamSlice, *stream)
	}

	return &LogBatch{
		Streams: streamSlice,
	}
}

// Helper functions
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func createStreamKey(labels map[string]string) string {
	data, _ := json.Marshal(labels)
	return string(data)
}

func timestampToNano(t time.Time) string {
	return string(rune(t.UnixNano()))
}
