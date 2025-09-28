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
	SemanticSearch   SemanticSearchConfig
	DefinitionSearch DefinitionSearchConfig
	ReferenceSearch  ReferenceSearchConfig
	KnowledgeSearch  KnowledgeSearchConfig
}

type SemanticSearchConfig struct {
	SearchEndpoint   string
	ApiReadyEndpoint string
	TopK             int
	ScoreThreshold   float64
}

type ReferenceSearchConfig struct {
	SearchEndpoint   string
	ApiReadyEndpoint string
}

type DefinitionSearchConfig struct {
	SearchEndpoint   string
	ApiReadyEndpoint string
}

type KnowledgeSearchConfig struct {
	SearchEndpoint   string
	ApiReadyEndpoint string
	TopK             int
	ScoreThreshold   float64
}

// LogConfig holds logging configuration
type LogConfig struct {
	LogFilePath          string
	LokiEndpoint         string
	LogScanIntervalSec   int
	ClassifyModel        string
	EnableClassification bool
}

type ContextCompressConfig struct {
	// Context compression enable flag
	EnableCompress bool
	// Context compression token threshold
	TokenThreshold int
	// Summary Model configuration
	SummaryModel               string
	SummaryModelTokenThreshold int
	// used recent user prompt messages nums
	RecentUserMsgUsedNums int
}

// Config holds all service configuration
type Config struct {
	// Server configuration
	Host string
	Port int

	// Tools configuration
	Tools ToolConfig

	// Logging configuration
	Log LogConfig

	// Context compression configuration
	ContextCompressConfig ContextCompressConfig

	//Department configuration
	DepartmentApiEndpoint string

	// Redis configuration
	Redis RedisConfig

	LLM LLMConfig
}
