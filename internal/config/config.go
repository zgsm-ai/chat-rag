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

	// Router configuration
	Router RouterConfig `mapstructure:"router" yaml:"router"`
}

// RouterConfig holds router related configuration
type RouterConfig struct {
	Enabled  bool           `mapstructure:"enabled" yaml:"enabled"`
	Strategy string         `mapstructure:"strategy" yaml:"strategy"`
	Semantic SemanticConfig `mapstructure:"semantic" yaml:"semantic"`
}

// SemanticConfig holds semantic router strategy configuration
type SemanticConfig struct {
	Analyzer        AnalyzerConfig        `mapstructure:"analyzer" yaml:"analyzer"`
	InputExtraction InputExtractionConfig `mapstructure:"inputExtraction" yaml:"inputExtraction"`
	Routing         RoutingConfig         `mapstructure:"routing" yaml:"routing"`
	RuleEngine      RuleEngineConfig      `mapstructure:"ruleEngine" yaml:"ruleEngine"`
}

// AnalyzerConfig only keeps model and timeoutMs per requirements
type AnalyzerConfig struct {
	Model          string `mapstructure:"model" yaml:"model"`
	TimeoutMs      int    `mapstructure:"timeoutMs" yaml:"timeoutMs"`
	TotalTimeoutMs int    `mapstructure:"totalTimeoutMs" yaml:"totalTimeoutMs"`
	MaxInputBytes  int    `mapstructure:"maxInputBytes" yaml:"maxInputBytes"`
	// Optional: override endpoint and token for analyzer-only requests
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint"`
	ApiToken string `mapstructure:"apiToken" yaml:"apiToken"`
	// Optional fields; ignored if empty
	PromptTemplate string               `mapstructure:"promptTemplate" yaml:"promptTemplate"`
	AnalysisLabels []string             `mapstructure:"analysisLabels" yaml:"analysisLabels"`
	DynamicMetrics DynamicMetricsConfig `mapstructure:"dynamicMetrics" yaml:"dynamicMetrics"`
}

// InputExtractionConfig controls how to extract input and history
type InputExtractionConfig struct {
	Protocol        string `mapstructure:"protocol" yaml:"protocol"`
	UserJoinSep     string `mapstructure:"userJoinSep" yaml:"userJoinSep"`
	StripCodeFences bool   `mapstructure:"stripCodeFences" yaml:"stripCodeFences"`
	CodeFenceRegex  string `mapstructure:"codeFenceRegex" yaml:"codeFenceRegex"`
	MaxUserMessages int    `mapstructure:"maxUserMessages" yaml:"maxUserMessages"`
	MaxHistoryBytes int    `mapstructure:"maxHistoryBytes" yaml:"maxHistoryBytes"`
	// MaxHistoryMessages limits how many history entries (after processing) can be included.
	// When >0, only the most recent N history items are kept.
	MaxHistoryMessages int `mapstructure:"maxHistoryMessages" yaml:"maxHistoryMessages"`
}

// RoutingConfig holds candidate model routing configuration
type RoutingConfig struct {
	Candidates        []RoutingCandidate `mapstructure:"candidates" yaml:"candidates"`
	MinScore          int                `mapstructure:"minScore" yaml:"minScore"`
	TieBreakOrder     []string           `mapstructure:"tieBreakOrder" yaml:"tieBreakOrder"`
	FallbackModelName string             `mapstructure:"fallbackModelName" yaml:"fallbackModelName"`
}

// RoutingCandidate defines a candidate model and its scores
type RoutingCandidate struct {
	ModelName string         `mapstructure:"modelName" yaml:"modelName"`
	Enabled   bool           `mapstructure:"enabled" yaml:"enabled"`
	Scores    map[string]int `mapstructure:"scores" yaml:"scores"`
}

// RuleEngineConfig is optional and configurable
type RuleEngineConfig struct {
	Enabled      bool     `mapstructure:"enabled" yaml:"enabled"`
	InlineRules  []string `mapstructure:"inlineRules" yaml:"inlineRules"`
	BodyPrefix   string   `mapstructure:"bodyPrefix" yaml:"bodyPrefix"`
	HeaderPrefix string   `mapstructure:"headerPrefix" yaml:"headerPrefix"`
}

// DynamicMetricsConfig controls dynamic metrics loading for candidate filtering
type DynamicMetricsConfig struct {
	Enabled     bool     `mapstructure:"enabled" yaml:"enabled"`
	RedisPrefix string   `mapstructure:"redisPrefix" yaml:"redisPrefix"`
	Metrics     []string `mapstructure:"metrics" yaml:"metrics"`
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
