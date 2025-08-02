package types

import (
	"fmt"
	"net/http"
)

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
	ErrMsgContextExceeded  = "The request exceeds the model's maximum context length. Please reduce the length of your input."

	ErrCodeModelServiceUnavailable = "chat-rag.model_service_unavailable"
	ErrMsgModelServiceUnavailable  = "Unable to access the AI model service. Please try again later."

	ErrCodeInernalError = "chat-rag.internal_error"
	ErrMsgInernalError  = "Internal Server Error. Please try again later."
)

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"statusCode,omitempty"`
}

func NewContextTooLongError() *APIError {
	return &APIError{
		Code:       ErrCodeContextExceeded,
		Message:    ErrMsgContextExceeded,
		Success:    false,
		StatusCode: http.StatusBadRequest,
	}
}

func NewModelServiceUnavailableError() *APIError {
	return &APIError{
		Code:       ErrCodeModelServiceUnavailable,
		Message:    ErrMsgModelServiceUnavailable,
		Success:    false,
		StatusCode: http.StatusServiceUnavailable,
	}
}

func NewHTTPStatusError(statusCode int, message string) *APIError {
	return &APIError{
		Code:       fmt.Sprintf("%d", statusCode),
		Message:    message,
		Success:    false,
		StatusCode: statusCode,
	}
}

func (e *APIError) Error() string {
	return fmt.Sprintf(`{"code":"%s","message":"%s","success":%v}`, e.Code, e.Message, e.Success)
}
