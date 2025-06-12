package types

const (
	// System role message
	RoleSystem = "system"

	// User role message
	RoleUser = "user"

	// AI assistant role message
	RoleAssistant = "assistant"
)

// ErrorType defines different types of errors
type ErrorType string

const (
	// ErrSemantic represents semantic processing errors
	ErrSemantic ErrorType = "SemanticError"

	// ErrSummary represents summary generation errors
	ErrSummary ErrorType = "SummaryError"

	// ErrExtra represents extra operation errors
	ErrExtra ErrorType = "ExtraError"
)

// PromptMode defines different types of chat
type PromptMode string

const (
	// Raw mode: No deep processing of user prompt, only necessary operations like compression
	Raw PromptMode = "raw"

	// Balanced mode: Considering both cost and performance, choosing a compromise approach
	// including rag and prompt compression
	Balanced PromptMode = "balanced"

	// Cost-first mode: Minimizing LLM calls and context size to save cost
	Cost PromptMode = "cost"

	// Performance-first mode: Maximizing output quality without considering cost
	Performance PromptMode = "performance"

	// Auto select mode: Default is balanced mode
	Auto PromptMode = "auto"
)
