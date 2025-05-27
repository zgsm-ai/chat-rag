package config

import "github.com/zeromicro/go-zero/rest"

// Config holds all service configuration
type Config struct {
	rest.RestConf

	// Model endpoints configuration
	MainModelEndpoint    string `json:",default=http://localhost:8000/v1/chat/completions"`
	SummaryModelEndpoint string `json:",default=http://localhost:8001/v1/chat/completions"`

	// Token processing configuration
	TokenThreshold int `json:",default=5000"`

	// Semantic API configuration
	SemanticApiEndpoint string `json:",default=http://localhost:8002/codebase-indexer/api/v1/semantics"`
	TopK                int    `json:",default=5"`

	// Feature flags
	EnableCompression bool `json:",default=true"`

	// Logging configuration
	LogFilePath        string `json:",default=logs/chat-rag.log"`
	LokiEndpoint       string `json:",default=http://localhost:3100/loki/api/v1/push"`
	LogBatchSize       int    `json:",default=100"`
	LogScanIntervalSec int    `json:",default=60"`

	// Model configuration
	SummaryModel string `json:",default=deepseek-chat"`
}
