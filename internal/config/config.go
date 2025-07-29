package config

// LLMConfig
type LLMConfig struct {
	Endpoint          string
	FuncCallingModels []string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// Config holds all service configuration
type Config struct {
	// Server configuration
	Host string
	Port int

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
	SystemPromptSplitStr string

	// used recent user prompt messages nums
	RecentUserMsgUsedNums int

	//Department configuration
	DepartmentApiEndpoint string

	// Redis configuration
	Redis RedisConfig

	LLM LLMConfig
}
