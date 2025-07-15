package types

import "fmt"

// ErrorType defines different types of errors
type ErrorType string

const (
	// ErrSemantic represents semantic processing errors
	ErrSemantic ErrorType = "SemanticError"

	// ErrSummary represents summary generation errors
	ErrSummary ErrorType = "SummaryError"

	// ErrApi represents dependent API errors
	ErrApiError ErrorType = "ApiError"

	// ErrServer represents internal server errors
	ErrServerError ErrorType = "ServerError"

	// ErrServer represents context length exceeded
	ErrContextExceeded ErrorType = "ContextLengthExceeded"

	// ErrExtra represents extra operation errors
	ErrExtra ErrorType = "ExtraError"
)

const (
	ErrCodeContextExceeded = "chat-rag.context_length_exceeded"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

func NewContextTooLongError() *APIError {
	return &APIError{
		Code:    ErrCodeContextExceeded,
		Message: "The request exceeds the model's maximum context length. Please reduce the length of your input.",
		Success: false,
	}
}

func (e *APIError) Error() string {
	return fmt.Sprintf(`{"code":"%s","message":"%s","success":%v}`, e.Code, e.Message, e.Success)
}
