package errors

import (
	"fmt"
	"strings"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrorTypeConfiguration ErrorType = "configuration"
	ErrorTypeConnection    ErrorType = "connection"
	ErrorTypeQuery         ErrorType = "query"
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeFormat        ErrorType = "format"
)

// QueryError represents a structured error with context and suggestions
type QueryError struct {
	Type        ErrorType `json:"type"`
	Message     string    `json:"message"`
	Details     string    `json:"details,omitempty"`
	Suggestions []string  `json:"suggestions,omitempty"`
	Cause       error     `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (e *QueryError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("[%s] %s", e.Type, e.Message))
	
	if e.Details != "" {
		parts = append(parts, fmt.Sprintf("Details: %s", e.Details))
	}
	
	if len(e.Suggestions) > 0 {
		parts = append(parts, fmt.Sprintf("Suggestions: %s", strings.Join(e.Suggestions, "; ")))
	}
	
	return strings.Join(parts, "\n")
}

// Unwrap returns the underlying error for error wrapping
func (e *QueryError) Unwrap() error {
	return e.Cause
}

// NewConfigurationError creates a new configuration-related error
func NewConfigurationError(message string, cause error, suggestions ...string) *QueryError {
	return &QueryError{
		Type:        ErrorTypeConfiguration,
		Message:     message,
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// NewConnectionError creates a new connection-related error
func NewConnectionError(message string, cause error, suggestions ...string) *QueryError {
	return &QueryError{
		Type:        ErrorTypeConnection,
		Message:     message,
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// NewQueryError creates a new query-related error
func NewQueryError(message string, cause error, suggestions ...string) *QueryError {
	return &QueryError{
		Type:        ErrorTypeQuery,
		Message:     message,
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// NewValidationError creates a new validation-related error
func NewValidationError(message string, cause error, suggestions ...string) *QueryError {
	return &QueryError{
		Type:        ErrorTypeValidation,
		Message:     message,
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// NewFormatError creates a new formatting-related error
func NewFormatError(message string, cause error, suggestions ...string) *QueryError {
	return &QueryError{
		Type:        ErrorTypeFormat,
		Message:     message,
		Suggestions: suggestions,
		Cause:       cause,
	}
}

// WrapWithSuggestions wraps an existing error with suggestions
func WrapWithSuggestions(err error, errorType ErrorType, message string, suggestions ...string) *QueryError {
	return &QueryError{
		Type:        errorType,
		Message:     message,
		Suggestions: suggestions,
		Cause:       err,
	}
}

// Common error messages and suggestions
var (
	// Connection error suggestions
	ConnectionSuggestions = []string{
		"Check if DynamoDB Local or LocalStack is running",
		"Verify the endpoint URL is correct",
		"Ensure the service is accessible from your network",
		"Try using 'docker ps' to check if LocalStack container is running",
	}

	// Configuration error suggestions
	ConfigSuggestions = []string{
		"Check if the config file exists at ~/.config/querydb/config.yaml",
		"Verify the YAML syntax is correct",
		"Ensure all required fields are present",
		"Run with --help to see configuration options",
	}

	// Table not found suggestions
	TableNotFoundSuggestions = []string{
		"Verify the table name is correct",
		"Check if the table exists in the specified region",
		"Ensure you have the correct permissions",
		"Try listing tables first to confirm the table exists",
	}

	// Authentication error suggestions
	AuthSuggestions = []string{
		"Check your AWS credentials",
		"For LocalStack, try using access_key_id: 'foo' and secret_access_key: 'bar'",
		"Verify the region is correct",
		"Ensure your credentials have DynamoDB permissions",
	}
)