package config

// Config holds all service configuration
type Config struct {
	// Server configuration
	Host string
	Port int

	// Model endpoints configuration
	LLMEndpoint string

	// Token processing configuration
	TokenThreshold int

	// Semantic API configuration
	SemanticApiEndpoint    string
	TopK                   int
	SemanticScoreThreshold float64

	// Logging configuration
	LogFilePath        string
	LokiEndpoint       string
	LogScanIntervalSec int

	// Model configuration
	SummaryModel               string
	SummaryModelTokenThreshold int
	ClassifyModel              string

	// Split system prompt
	SystemPromptSplitter string

	// used recent user prompt messages nums
	RecentUserMsgUsedNums int

	// oneapi Authorization
	OneApiAuthorization string
}
