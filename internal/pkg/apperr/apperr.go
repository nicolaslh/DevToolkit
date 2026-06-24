// Package apperr defines the unified error model shared across DevToolkit services.
package apperr

import "fmt"

// Code is a machine-readable error category the frontend uses to pick a prompt style.
type Code string

const (
	InvalidInput Code = "INVALID_INPUT"
	TooLarge     Code = "TOO_LARGE"
	ParseError   Code = "PARSE_ERROR"
	Network      Code = "NETWORK_ERROR"
	Timeout      Code = "TIMEOUT"
	Clipboard    Code = "CLIPBOARD_ERROR"
	Unsupported  Code = "UNSUPPORTED"
)

// AppError carries a user-facing (Chinese) message plus a machine-readable code.
type AppError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string { return e.Message }

// New builds an AppError.
func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// Newf builds an AppError with a formatted message.
func Newf(code Code, format string, args ...any) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, args...)}
}
