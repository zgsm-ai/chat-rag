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
	// Global switch to control whether all tools are disabled, default is false
	DisableTools bool
	// Control which agents in which modes cannot use tools
	DisabledAgents map[string][]string

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

type PreciseContextConfig struct {
	// AgentsMatch configuration
	AgentsMatch []AgentMatchConfig
	// filter "environment_details" user prompt in context
	EnableEnvDetailsFilter bool
	// Control which agents in which modes cannot use ModesChange
	DisabledModesChangeAgents map[string][]string
}

// AgentMatchConfig holds configuration for a specific agent matching
type AgentMatchConfig struct {
	AgentName string
	MatchKey  string
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

	// Context handling configuration
	ContextCompressConfig ContextCompressConfig
	PreciseContextConfig  PreciseContextConfig

	//Department configuration
	DepartmentApiEndpoint string

	// Redis configuration
	Redis RedisConfig

	LLM LLMConfig
}

// AgentConfig holds configuration for a specific agent
type AgentConfig struct {
	MatchAgents []string `mapstructure:"match_agents"`
	MatchModes  []string `mapstructure:"match_modes"`
	Rules       string   `mapstructure:"rules"`
}

// RulesConfig holds the rules configuration for agents
type RulesConfig struct {
	Agents []AgentConfig `yaml:"agents"`
}
