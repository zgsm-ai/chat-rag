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

type ToolConfig struct {
	SemanticSearch SemanticSearchConfig
	RelationSearch RelationSearchConfig
}

type SemanticSearchConfig struct {
	SearchEndpoint   string
	ApiReadyEndpoint string
	TopK             int
	ScoreThreshold   float64
}

type RelationSearchConfig struct {
	SearchEndpoint string
}

// Config holds all service configuration
type Config struct {
	// Server configuration
	Host string
	Port int

	// Token processing configuration
	TokenThreshold int

	// Tools configuration
	Tools ToolConfig

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
