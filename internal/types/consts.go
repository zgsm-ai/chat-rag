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

// ChatMode defines different types of chat
type ChatMode string

const (
	// Direct chat without any extra operations
	Direct ChatMode = "direct"

	// Rag processing, including rag and prompt compression
	Rag ChatMode = "rag"
)
